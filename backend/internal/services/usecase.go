package services_scan

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/patdyyew-emran/phishing-detector/internal/auth"
	"github.com/patdyyew-emran/phishing-detector/internal/infra"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func NewServices(
	db *gorm.DB,
	redis *redis.Client,
	publisher MessagePublisher,
	log *slog.Logger,
) *Scan {
	if log == nil {
		log = slog.Default()
	}

	return &Scan{
		db:        db,
		redis:     redis,
		publisher: publisher,
		log:       log,
		subject:   ScanSubject,
	}
}

func NewServicesWithSubject(
	db *gorm.DB,
	redis *redis.Client,
	publisher MessagePublisher,
	subject string,
	log *slog.Logger,
) *Scan {
	if log == nil {
		log = slog.Default()
	}

	subject = strings.TrimSpace(subject)
	if subject == "" {
		subject = ScanSubject
	}

	return &Scan{
		db:        db,
		redis:     redis,
		publisher: publisher,
		log:       log,
		subject:   subject,
	}
}

func (s *Scan) CreateScan(c *fiber.Ctx) error {
	if s == nil || s.db == nil {
		return fiber.NewError(
			http.StatusServiceUnavailable,
			"Scan service is unavailable",
		)
	}

	var body struct {
		URL string `json:"url"`
	}

	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(
			http.StatusBadRequest,
			"Invalid request body",
		)
	}

	rawURL := strings.TrimSpace(body.URL)

	if rawURL == "" {
		return fiber.NewError(
			http.StatusBadRequest,
			"URL is required",
		)
	}

	ownerID, err := auth.UserIDFromContext(c)
	if err != nil {
		return fiber.NewError(
			http.StatusUnauthorized,
			"Authentication required",
		)
	}

	if s.publisher == nil {
		return fiber.NewError(
			http.StatusServiceUnavailable,
			"Scan queue is unavailable",
		)
	}

	requestCtx := c.UserContext()
	if requestCtx == nil {
		requestCtx = context.Background()
	}

	created := infra.ScanModel{
		UserID:    ownerID,
		URL:       rawURL,
		Status:    ScanStatusPending,
		RiskLevel: "unknown",
	}

	// Persist Scan and AuditLog atomically.
	dbCtx, dbCancel := context.WithTimeout(
		requestCtx,
		5*time.Second,
	)

	err = s.db.WithContext(dbCtx).Transaction(
		func(tx *gorm.DB) error {
			if err := tx.Create(&created).Error; err != nil {
				return fmt.Errorf("create scan: %w", err)
			}

			audit := infra.AuditLogModel{
				UserID:   ownerID,
				Action:   "scan.created",
				Entity:   "scan",
				EntityID: created.ID,
			}

			if err := tx.Create(&audit).Error; err != nil {
				return fmt.Errorf("create audit log: %w", err)
			}

			return nil
		},
	)

	dbCancel()

	if err != nil {
		return err
	}

	// Publish the persisted Scan ID to JetStream.
	publishCtx, publishCancel := context.WithTimeout(
		requestCtx,
		5*time.Second,
	)

	publishErr := s.publisher.Publish(
		publishCtx,
		s.subject,
		[]byte(strconv.FormatUint(uint64(created.ID), 10)),
	)

	publishCancel()

	if publishErr != nil {
		// Use an independent context for the failure update.
		updateCtx, updateCancel := context.WithTimeout(
			context.Background(),
			3*time.Second,
		)

		updateErr := s.db.WithContext(updateCtx).
			Model(&created).
			Updates(map[string]interface{}{
				"status": "failed",
				"error":  "Failed to publish scan task",
			}).Error

		updateCancel()

		if updateErr != nil && s.log != nil {
			s.log.Error(
				"failed to update scan after publish error",
				"scanID", created.ID,
				"publishError", publishErr,
				"updateError", updateErr,
			)
		}

		return fiber.NewError(
			http.StatusServiceUnavailable,
			"Scan queue is unavailable",
		)
	}

	return c.Status(http.StatusAccepted).JSON(created)
}

func (s *Scan) GetScan(c *fiber.Ctx) error {
	scanID, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil || scanID == 0 || uint64(uint(scanID)) != scanID {
		return fiber.NewError(
			http.StatusBadRequest,
			"Invalid scan ID",
		)
	}

	query, err := s.scanScope(c)
	if err != nil {
		return err
	}

	var item infra.ScanModel

	err = query.
		Preload("Indicators").
		Where("scan_models.id = ?", uint(scanID)).
		First(&item).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fiber.NewError(
			http.StatusNotFound,
			"Scan not found",
		)
	}

	if err != nil {
		return err
	}

	return c.JSON(item)
}

func (s *Scan) ListScans(c *fiber.Ctx) error {
	query, err := s.scanScope(c)
	if err != nil {
		return err
	}

	limit, err := strconv.Atoi(c.Query("limit", "20"))
	if err != nil || limit < 1 || limit > 100 {
		limit = 20
	}

	var total int64

	if err := query.Count(&total).Error; err != nil {
		return err
	}

	var items []infra.ScanModel

	if err := query.
		Preload("Indicators").
		Order("created_at DESC").
		Limit(limit).
		Find(&items).Error; err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"items": items,
		"total": total,
		"limit": limit,
	})
}

func (s *Scan) Stats(c *fiber.Ctx) error {
	query, err := s.scanScope(c)
	if err != nil {
		return err
	}

	var total int64
	var high int64

	if err := query.Count(&total).Error; err != nil {
		return err
	}

	if err := query.
		Where(
			"risk_level IN ?",
			[]string{"high", "critical"},
		).
		Count(&high).Error; err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"totalScans":    total,
		"highRiskScans": high,
	})
}

func ErrorHandler(c *fiber.Ctx, err error) error {
	code := http.StatusInternalServerError
	message := "Içerki serwer ýalňyşlygy"

	var fiberError *fiber.Error
	if errors.As(err, &fiberError) {
		code = fiberError.Code
		message = fiberError.Message
	}

	return c.Status(code).JSON(fiber.Map{"error": message})
}

func (s *Scan) scanScope(c *fiber.Ctx) (*gorm.DB, error) {
	userID, err := auth.UserIDFromContext(c)
	if err != nil {
		return nil, fiber.NewError(
			http.StatusUnauthorized,
			"Authentication required",
		)
	}

	role, err := auth.UserRoleFromContext(c)
	if err != nil {
		return nil, fiber.NewError(
			http.StatusUnauthorized,
			"Authentication required",
		)
	}

	query := s.db.
		WithContext(c.UserContext()).
		Model(&infra.ScanModel{})

	if role != auth.RoleAdmin {
		query = query.Where("user_id = ?", userID)
	}

	return query, nil
}
