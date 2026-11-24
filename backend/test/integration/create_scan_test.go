package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	appRouter "github.com/patdyyew-emran/phishing-detector/internal/app"
	authpkg "github.com/patdyyew-emran/phishing-detector/internal/auth"
	"github.com/patdyyew-emran/phishing-detector/internal/infra"
	services_scan "github.com/patdyyew-emran/phishing-detector/internal/services"
	"gorm.io/gorm"
)

type recordingPublisher struct {
	ctxErr  error
	subject string
	data    []byte
	err     error
	calls   int
}

func (p *recordingPublisher) Publish(
	ctx context.Context,
	subject string,
	data []byte,
) error {
	p.calls++
	p.subject = subject
	p.data = append([]byte(nil), data...)

	if ctx == nil {
		return errors.New("publish context is nil")
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	return p.err
}

func newCreateScanRouter(
	db *gorm.DB,
	publisher services_scan.MessagePublisher,
) *fiber.App {
	scanService := services_scan.NewServices(
		db,
		nil,
		publisher,
		slog.Default(),
	)

	return appRouter.NewRouter(appRouter.Dependencies{
		DB:          db,
		ScanService: scanService,
		JWTSecret:   integrationJWTSecret,
	})
}

func registerCreatedScanCleanup(
	t *testing.T,
	db *gorm.DB,
	userID uint,
	rawURL string,
) {
	t.Helper()

	t.Cleanup(func() {
		var scans []infra.ScanModel

		if err := db.
			Where("user_id = ? AND url = ?", userID, rawURL).
			Find(&scans).Error; err != nil {
			t.Errorf("find scans for cleanup: %v", err)
			return
		}

		for _, scan := range scans {
			if err := db.
				Where(
					"user_id = ? AND entity = ? AND entity_id = ?",
					userID,
					"scan",
					scan.ID,
				).
				Delete(&infra.AuditLogModel{}).Error; err != nil {
				t.Errorf("delete audit log for scan %d: %v", scan.ID, err)
			}

			if err := db.Delete(&scan).Error; err != nil {
				t.Errorf("delete scan %d: %v", scan.ID, err)
			}
		}
	})
}

func TestCreateScanUsesAuthenticatedUserID(t *testing.T) {
	db := openTestDB(t)

	userA := createOwnershipUser(
		t,
		db,
		"create-scan-owner",
		authpkg.RoleUser,
	)

	userB := createOwnershipUser(
		t,
		db,
		"create-scan-attacker",
		authpkg.RoleUser,
	)

	token := createOwnershipToken(t, userA)

	publisher := &recordingPublisher{}

	router := newCreateScanRouter(db, publisher)

	rawURL := "https://example.test/login"

	registerCreatedScanCleanup(
		t,
		db,
		userA.ID,
		rawURL,
	)

	requestBody := fmt.Sprintf(
		`{"url":%q,"userId":%d,"user_id":%d}`,
		rawURL,
		userB.ID,
		userB.ID,
	)

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/scans/",
		strings.NewReader(requestBody),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := router.Test(req)
	if err != nil {
		t.Fatalf("execute create scan request: %v", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	if publisher.calls != 1 {
		t.Fatalf(
			"expected publisher to be called once, got %d",
			publisher.calls,
		)
	}

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf(
			"expected 202, got %d: %s",
			resp.StatusCode,
			responseBody,
		)
	}

	var created infra.ScanModel

	if err := json.Unmarshal(responseBody, &created); err != nil {
		t.Fatalf("decode created scan: %v", err)
	}

	if created.ID == 0 {
		t.Fatal("created scan ID is zero")
	}

	if created.UserID != userA.ID {
		t.Fatalf(
			"scan owner must be authenticated user %d, got %d",
			userA.ID,
			created.UserID,
		)
	}

	if created.UserID == userB.ID {
		t.Fatal("server accepted attacker-supplied user ID")
	}

	if created.Status != services_scan.ScanStatusPending {
		t.Fatalf("expected pending status, got %q", created.Status)
	}

	var stored infra.ScanModel

	if err := db.First(&stored, created.ID).Error; err != nil {
		t.Fatalf("find persisted scan: %v", err)
	}

	if stored.UserID != userA.ID {
		t.Fatalf(
			"persisted scan owner must be %d, got %d",
			userA.ID,
			stored.UserID,
		)
	}

	var audit infra.AuditLogModel

	if err := db.
		Where(
			"user_id = ? AND action = ? AND entity = ? AND entity_id = ?",
			userA.ID,
			"scan.created",
			"scan",
			created.ID,
		).
		First(&audit).Error; err != nil {
		t.Fatalf("find scan audit log: %v", err)
	}

	if audit.UserID != userA.ID {
		t.Fatalf("audit log has incorrect user ID: %d", audit.UserID)
	}

	if publisher.subject != services_scan.ScanSubject {
		t.Fatalf(
			"expected subject %q, got %q",
			services_scan.ScanSubject,
			publisher.subject,
		)
	}

	expectedPayload := strconv.FormatUint(
		uint64(created.ID),
		10,
	)

	if string(publisher.data) != expectedPayload {
		t.Fatalf(
			"expected NATS payload %q, got %q",
			expectedPayload,
			string(publisher.data),
		)
	}
}

func TestCreateScanRequiresAuthentication(t *testing.T) {
	db := openTestDB(t)

	publisher := &recordingPublisher{}
	router := newCreateScanRouter(db, publisher)

	rawURL := "https://example.test/unauthorized"

	req := httptest.NewRequest(
		http.MethodPost,
		"/api/scans/",
		strings.NewReader(fmt.Sprintf(`{"url":%q}`, rawURL)),
	)

	req.Header.Set("Content-Type", "application/json")

	resp, err := router.Test(req)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)

		t.Fatalf(
			"expected 401, got %d: %s",
			resp.StatusCode,
			body,
		)
	}

	if publisher.calls != 0 {
		t.Fatalf(
			"publisher must not be called, got %d calls",
			publisher.calls,
		)
	}

	var count int64

	if err := db.Model(&infra.ScanModel{}).
		Where("url = ?", rawURL).
		Count(&count).Error; err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Fatalf("expected no persisted scans, got %d", count)
	}
}

func TestCreateScanRejectsInvalidRequestBody(t *testing.T) {
	db := openTestDB(t)

	user := createOwnershipUser(
		t,
		db,
		"create-scan-invalid-body",
		authpkg.RoleUser,
	)

	token := createOwnershipToken(t, user)

	publisher := &recordingPublisher{}

	router := newCreateScanRouter(db, publisher)

	tests := []struct {
		name string
		body string
	}{
		{
			name: "malformed JSON",
			body: `{`,
		},
		{
			name: "JSON null",
			body: `null`,
		},
		{
			name: "empty object",
			body: `{}`,
		},
		{
			name: "missing URL",
			body: `{"name":"example"}`,
		},
		{
			name: "empty URL",
			body: `{"url":""}`,
		},
		{
			name: "whitespace URL",
			body: `{"url":"   "}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(
				http.MethodPost,
				"/api/scans/",
				strings.NewReader(tt.body),
			)

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)

			resp, err := router.Test(req)
			if err != nil {
				t.Fatalf("execute request: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf(
					"expected 400, got %d",
					resp.StatusCode,
				)
			}
		})
	}

	if publisher.calls != 0 {
		t.Fatalf(
			"publisher must not be called, got %d calls",
			publisher.calls,
		)
	}
}

func TestCreateScanMarksScanFailedWhenPublishFails(t *testing.T) {
	db := openTestDB(t)

	user := createOwnershipUser(
		t,
		db,
		"create-scan-publish-error",
		authpkg.RoleUser,
	)

	token := createOwnershipToken(t, user)

	publishErr := errors.New("NATS connection unavailable")

	publisher := &recordingPublisher{
		err: publishErr,
	}

	router := newCreateScanRouter(db, publisher)

	rawURL := "https://example.test/publish-failure"

	registerCreatedScanCleanup(
		t,
		db,
		user.ID,
		rawURL,
	)

	resp, body := performRequest(
		t,
		router,
		http.MethodPost,
		token,
		"/api/scans/",
		fmt.Sprintf(`{"url":%q}`, rawURL),
	)

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf(
			"expected 503, got %d: %s",
			resp.StatusCode,
			body,
		)
	}

	if publisher.calls != 1 {
		t.Fatalf(
			"expected one publish attempt, got %d",
			publisher.calls,
		)
	}

	var stored infra.ScanModel

	if err := db.
		Where("user_id = ? AND url = ?", user.ID, rawURL).
		First(&stored).Error; err != nil {
		t.Fatalf("find failed scan: %v", err)
	}

	if stored.Status != "failed" {
		t.Fatalf(
			"expected failed status, got %q",
			stored.Status,
		)
	}

	if stored.Error != "Failed to publish scan task" {
		t.Fatalf(
			"unexpected scan error: %q",
			stored.Error,
		)
	}

	var audit infra.AuditLogModel

	if err := db.
		Where(
			"user_id = ? AND action = ? AND entity_id = ?",
			user.ID,
			"scan.created",
			stored.ID,
		).
		First(&audit).Error; err != nil {
		t.Fatalf("expected audit log to be persisted: %v", err)
	}
}

func performCreateScanRequest(
	t *testing.T,
	router *fiber.App,
	method string,
	path string,
	token string,
	body string,
) (int, []byte) {
	t.Helper()

	req := httptest.NewRequest(
		method,
		path,
		strings.NewReader(body),
	)

	req.Header.Set("Content-Type", "application/json")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := router.Test(req)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	return resp.StatusCode, responseBody
}

func TestCreateScanReturns503WhenPublisherIsNil(t *testing.T) {
	db := openTestDB(t)

	user := createOwnershipUser(
		t,
		db,
		"create-scan-no-publisher",
		authpkg.RoleUser,
	)

	token := createOwnershipToken(t, user)

	router := newCreateScanRouter(db, nil)

	rawURL := "https://example.test/no-publisher"

	status, body := performCreateScanRequest(
		t,
		router,
		http.MethodPost,
		"/api/scans/",
		token,
		fmt.Sprintf(`{"url":%q}`, rawURL),
	)

	if status != http.StatusServiceUnavailable {
		t.Fatalf("expected 500, got %d: %s", status, body)
	}

	var count int64
	if err := db.Model(&infra.ScanModel{}).
		Where("user_id = ? AND url = ?", user.ID, rawURL).
		Count(&count).Error; err != nil {
		t.Fatal(err)
	}

	if count != 0 {
		t.Fatalf("expected no scan to be created, got %d", count)
	}
}
