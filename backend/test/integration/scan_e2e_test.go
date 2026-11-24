package integration_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	appRouter "github.com/patdyyew-emran/phishing-detector/internal/app"
	"github.com/patdyyew-emran/phishing-detector/internal/auth"
	"github.com/patdyyew-emran/phishing-detector/internal/infra"
	services_scan "github.com/patdyyew-emran/phishing-detector/internal/services"
)

func TestCreateScanEndToEndThroughJetStreamWorker(t *testing.T) {
	db := openTestDB(t)
	fixture := newWorkerJetStreamFixture(t)

	// Use the real JetStream publisher.
	publisher, err := infra.NewJetStreamPublisher(fixture.js)
	if err != nil {
		t.Fatalf("create JetStream publisher: %v", err)
	}

	user := createOwnershipUser(
		t,
		db,
		"scan-e2e",
		auth.RoleUser,
	)

	token := createOwnershipToken(t, user)

	rawURL := "http://192.0.2.1/login"

	registerCreatedScanCleanup(
		t,
		db,
		user.ID,
		rawURL,
	)

	// The worker processes messages asynchronously.
	workerService := services_scan.NewServicesWithSubject(
		db,
		nil,
		publisher,
		fixture.subject,
		slog.Default(),
	)

	workerCtx, cancelWorker := context.WithCancel(
		context.Background(),
	)

	// The HTTP service creates Scan and publishes its ID.
	router := appRouter.NewRouter(appRouter.Dependencies{
		DB:          db,
		ScanService: workerService,
		JWTSecret:   integrationJWTSecret,
	})

	consumerErrors := make(chan error, 1)

	consumeCtx, err := fixture.consumer.Consume(
		func(message jetstream.Msg) {
			workerService.HandleJetStreamMessage(
				workerCtx,
				message,
			)
		},
		jetstream.PullMaxMessages(1),
		jetstream.ConsumeErrHandler(
			func(
				_ jetstream.ConsumeContext,
				err error,
			) {
				select {
				case consumerErrors <- err:
				default:
				}
			},
		),
	)
	if err != nil {
		cancelWorker()
		t.Fatalf("start JetStream consumer: %v", err)
	}

	if consumeCtx == nil {
		cancelWorker()
		t.Fatal("JetStream consume context is nil")
	}

	// Stop the consumer before the fixture deletes the stream.
	t.Cleanup(func() {
		consumeCtx.Stop()
		cancelWorker()

		select {
		case <-consumeCtx.Closed():
		case <-time.After(3 * time.Second):
			t.Error("JetStream consumer did not stop")
		}
	})

	// Send a real authenticated HTTP request.
	status, body := performCreateScanRequest(
		t,
		router,
		http.MethodPost,
		"/api/scans/",
		token,
		fmt.Sprintf(`{"url":%q}`, rawURL),
	)

	if status != http.StatusAccepted {
		t.Fatalf(
			"expected HTTP 202, got %d: %s",
			status,
			body,
		)
	}

	var created infra.ScanModel

	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatalf("decode created Scan: %v", err)
	}

	if created.ID == 0 {
		t.Fatal("created Scan ID is zero")
	}

	if created.UserID != user.ID {
		t.Fatalf(
			"expected owner %d, got %d",
			user.ID,
			created.UserID,
		)
	}

	// Wait for the asynchronous worker to finish processing.
	deadline := time.Now().Add(10 * time.Second)

	var result infra.ScanModel

	for {
		select {
		case consumerErr := <-consumerErrors:
			t.Fatalf("JetStream consumer error: %v", consumerErr)
		default:
		}

		if err := db.
			Preload("Indicators").
			First(&result, created.ID).Error; err != nil {
			t.Fatalf("load processed Scan: %v", err)
		}

		if result.Status == services_scan.ScanStatusCompleted {
			break
		}

		if result.Status == services_scan.ScanStatusFailed {
			t.Fatalf(
				"Scan processing failed: %s",
				result.Error,
			)
		}

		if time.Now().After(deadline) {
			t.Fatalf(
				"timed out waiting for Scan completion; status=%q",
				result.Status,
			)
		}

		time.Sleep(50 * time.Millisecond)
	}

	// Verify persisted analysis.
	if result.NormalizedURL != rawURL {
		t.Fatalf(
			"expected normalized URL %q, got %q",
			rawURL,
			result.NormalizedURL,
		)
	}

	if result.Score != 62 {
		t.Fatalf("expected score 62, got %d", result.Score)
	}

	if result.RiskLevel != "high" {
		t.Fatalf(
			"expected risk level high, got %q",
			result.RiskLevel,
		)
	}

	if len(result.Indicators) != 3 {
		t.Fatalf(
			"expected 3 indicators, got %d",
			len(result.Indicators),
		)
	}

	if result.Error != "" {
		t.Fatalf("expected empty error, got %q", result.Error)
	}

	// Verify AuditLog created by the HTTP request.
	var audit infra.AuditLogModel

	if err := db.
		Where(
			"user_id = ? AND action = ? AND entity = ? AND entity_id = ?",
			user.ID,
			"scan.created",
			"scan",
			created.ID,
		).
		First(&audit).Error; err != nil {
		t.Fatalf("load Scan audit log: %v", err)
	}

	// Wait until JetStream confirms the message.
	ackDeadline := time.Now().Add(3 * time.Second)

	for {
		info, err := fixture.consumer.Info(context.Background())
		if err != nil {
			t.Fatalf("get consumer info: %v", err)
		}

		if info.NumAckPending == 0 && info.NumPending == 0 {
			break
		}

		if time.Now().After(ackDeadline) {
			t.Fatalf(
				"message was not acknowledged: ack_pending=%d pending=%d",
				info.NumAckPending,
				info.NumPending,
			)
		}

		time.Sleep(50 * time.Millisecond)
	}
}
