package services_scan

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/patdyyew-emran/phishing-detector/internal/infra"
	"gorm.io/gorm"
)

const (
	scanProcessingTimeout   = 20 * time.Second
	scanProcessingLease     = 2 * time.Minute
	scanClaimReleaseTimeout = 3 * time.Second
	scanRetryDelay          = 5 * time.Second
	scanAckTimeout          = 3 * time.Second
)

var (
	errScanAlreadyProcessing = errors.New("scan is already processing")
	errInvalidScanStatus     = errors.New("scan has unsupported status")
)

// HandleJetStreamMessage handles one message delivered by JetStream.
// ACK/NAK/TERM decisions are made here.
func (s *Scan) HandleJetStreamMessage(
	workerCtx context.Context,
	message jetstream.Msg,
) {
	logger := s.workerLogger()

	if message == nil {
		logger.Error("received nil JetStream message")
		return
	}

	if s == nil || s.db == nil {
		logger.Error("scan service is unavailable")

		if err := message.NakWithDelay(scanRetryDelay); err != nil {
			logger.Error("failed to NAK message", "error", err)
		}

		return
	}

	if workerCtx == nil {
		workerCtx = context.Background()
	}

	rawID := strings.TrimSpace(string(message.Data()))

	scanID, err := strconv.ParseUint(rawID, 10, 64)
	if err != nil || scanID == 0 || uint64(uint(scanID)) != scanID {
		logger.Warn(
			"received invalid scan message",
			"payload", rawID,
		)

		if termErr := message.Term(); termErr != nil {
			logger.Error(
				"failed to terminate invalid message",
				"error", termErr,
			)
		}

		return
	}

	ctx, cancel := context.WithTimeout(
		workerCtx,
		scanProcessingTimeout,
	)

	err = s.processScan(ctx, uint(scanID))
	cancel()

	switch {
	case err == nil:
		s.ackJetStreamMessage(message, uint(scanID))

	case errors.Is(err, gorm.ErrRecordNotFound):
		logger.Warn(
			"scan referenced by message does not exist",
			"scanID", scanID,
		)

		if termErr := message.Term(); termErr != nil {
			logger.Error("failed to terminate message", "error", termErr)
		}

	case errors.Is(err, errInvalidScanStatus):
		logger.Error(
			"scan has invalid status",
			"scanID", scanID,
			"error", err,
		)

		if termErr := message.Term(); termErr != nil {
			logger.Error("failed to terminate message", "error", termErr)
		}

	case errors.Is(err, errScanAlreadyProcessing):
		logger.Debug(
			"scan is already being processed",
			"scanID", scanID,
		)

		if nakErr := message.NakWithDelay(scanRetryDelay); nakErr != nil {
			logger.Error("failed to NAK message", "error", nakErr)
		}

	default:
		logger.Error(
			"scan processing failed; message will be retried",
			"scanID", scanID,
			"error", err,
		)

		if nakErr := message.NakWithDelay(scanRetryDelay); nakErr != nil {
			logger.Error("failed to NAK message", "error", nakErr)
		}
	}
}

// processScan claims and processes one Scan.
// A completed or failed Scan is treated as an already processed message.
func (s *Scan) processScan(
	ctx context.Context,
	scanID uint,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var item infra.ScanModel

	if err := s.db.WithContext(ctx).
		First(&item, scanID).Error; err != nil {
		return fmt.Errorf("load scan %d: %w", scanID, err)
	}

	switch item.Status {
	case ScanStatusCompleted, ScanStatusFailed:
		// Duplicate delivery: already processed.
		return nil

	case ScanStatusPending, ScanStatusProcessing:
		// Continue to atomic claim.

	default:
		return fmt.Errorf(
			"%w: %q",
			errInvalidScanStatus,
			item.Status,
		)
	}

	// Every successful claim gets a new unique token.
	processingToken, err := newProcessingToken()
	if err != nil {
		return err
	}

	staleBefore := time.Now().UTC().Add(-scanProcessingLease)

	claim := s.db.WithContext(ctx).
		Model(&infra.ScanModel{}).
		Where(
			"id = ? AND (status = ? OR (status = ? AND updated_at < ?))",
			scanID,
			ScanStatusPending,
			ScanStatusProcessing,
			staleBefore,
		).
		Updates(map[string]interface{}{
			"status":           ScanStatusProcessing,
			"processing_token": processingToken,
			"error":            "",
		})

	if claim.Error != nil {
		return fmt.Errorf("claim scan %d: %w", scanID, claim.Error)
	}

	if claim.RowsAffected == 0 {
		var current infra.ScanModel

		if err := s.db.WithContext(ctx).
			First(&current, scanID).Error; err != nil {
			return fmt.Errorf("reload scan %d: %w", scanID, err)
		}

		switch current.Status {
		case ScanStatusCompleted, ScanStatusFailed:
			return nil

		case ScanStatusPending, ScanStatusProcessing:
			return errScanAlreadyProcessing

		default:
			return fmt.Errorf(
				"%w: %q",
				errInvalidScanStatus,
				current.Status,
			)
		}
	}

	if err := ctx.Err(); err != nil {
		s.releaseScanClaim(scanID, processingToken)
		return err
	}

	analysis, err := s.Analyze(ctx, item.URL)

	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			s.releaseScanClaim(scanID, processingToken)
			return ctxErr
		}

		if errors.Is(err, context.Canceled) ||
			errors.Is(err, context.DeadlineExceeded) {

			s.releaseScanClaim(scanID, processingToken)
			return err
		}

		if saveErr := s.markScanFailed(
			ctx,
			scanID,
			processingToken,
			err,
		); saveErr != nil {
			s.releaseScanClaim(scanID, processingToken)

			return fmt.Errorf(
				"save failed status for scan %d: %w",
				scanID,
				saveErr,
			)
		}

		s.workerLogger().Warn(
			"scan analysis rejected URL",
			"scanID", scanID,
			"error", err,
		)

		return nil
	}

	if err := ctx.Err(); err != nil {
		s.releaseScanClaim(scanID, processingToken)
		return err
	}

	if err := s.persistScanAnalysis(
		ctx,
		scanID,
		processingToken,
		analysis,
	); err != nil {
		s.releaseScanClaim(scanID, processingToken)

		return fmt.Errorf(
			"persist analysis for scan %d: %w",
			scanID,
			err,
		)
	}

	s.workerLogger().Info(
		"scan processing completed",
		"scanID", scanID,
		"score", analysis.Score,
		"riskLevel", analysis.RiskLevel,
	)

	return nil
}

func (s *Scan) markScanFailed(
	ctx context.Context,
	scanID uint,
	processingToken string,
	cause error,
) error {
	result := s.db.WithContext(ctx).
		Model(&infra.ScanModel{}).
		Where(
			"id = ? AND status = ? AND processing_token = ?",
			scanID,
			ScanStatusProcessing,
			processingToken,
		).
		Updates(map[string]interface{}{
			"status":           ScanStatusFailed,
			"error":            cause.Error(),
			"processing_token": "",
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected != 1 {
		return errors.New(
			"scan claim was lost before failure update",
		)
	}

	return nil
}

func (s *Scan) persistScanAnalysis(
	ctx context.Context,
	scanID uint,
	processingToken string,
	analysis Analysis,
) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where("scan_id = ?", scanID).
			Delete(&infra.IndicatorModel{}).Error; err != nil {
			return fmt.Errorf("delete old indicators: %w", err)
		}

		indicators := make(
			[]infra.IndicatorModel,
			len(analysis.Indicators),
		)

		copy(indicators, analysis.Indicators)

		for i := range indicators {
			indicators[i].ID = 0
			indicators[i].ScanID = scanID
		}

		if len(indicators) > 0 {
			if err := tx.Create(&indicators).Error; err != nil {
				return fmt.Errorf("create indicators: %w", err)
			}
		}

		updated := tx.
			Model(&infra.ScanModel{}).
			Where(
				"id = ? AND status = ? AND processing_token = ?",
				scanID,
				ScanStatusProcessing,
				processingToken,
			).
			Updates(map[string]interface{}{
				"normalized_url":   analysis.NormalizedURL,
				"status":           ScanStatusCompleted,
				"score":            analysis.Score,
				"risk_level":       analysis.RiskLevel,
				"error":            "",
				"processing_token": "",
			})

		if updated.Error != nil {
			return fmt.Errorf("update scan result: %w", updated.Error)
		}

		if updated.RowsAffected != 1 {
			return errors.New(
				"scan claim was lost before result update",
			)
		}

		return nil
	})
}

// releaseScanClaim returns a Scan to pending after a temporary error.
// A separate context is used because the processing context may be canceled.
func (s *Scan) releaseScanClaim(
	scanID uint,
	processingToken string,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		scanClaimReleaseTimeout,
	)
	defer cancel()

	result := s.db.WithContext(ctx).
		Model(&infra.ScanModel{}).
		Where(
			"id = ? AND status = ? AND processing_token = ?",
			scanID,
			ScanStatusProcessing,
			processingToken,
		).
		Updates(map[string]interface{}{
			"status":           ScanStatusPending,
			"error":            "",
			"processing_token": "",
		})

	if result.Error != nil {
		s.workerLogger().Error(
			"failed to release scan processing claim",
			"scanID", scanID,
			"error", result.Error,
		)
	}
}

func (s *Scan) ackJetStreamMessage(
	message jetstream.Msg,
	scanID uint,
) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		scanAckTimeout,
	)
	defer cancel()

	if err := message.DoubleAck(ctx); err != nil {
		s.workerLogger().Error(
			"failed to acknowledge scan message",
			"scanID", scanID,
			"error", err,
		)
	}
}

func (s *Scan) workerLogger() *slog.Logger {
	if s != nil && s.log != nil {
		return s.log
	}

	return slog.Default()
}

func newProcessingToken() (string, error) {
	var raw [16]byte

	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate processing token: %w", err)
	}

	return hex.EncodeToString(raw[:]), nil
}
