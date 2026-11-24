package integration_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/patdyyew-emran/phishing-detector/internal/auth"
	"github.com/patdyyew-emran/phishing-detector/internal/infra"
	services_scan "github.com/patdyyew-emran/phishing-detector/internal/services"
	"gorm.io/gorm"
)

type workerJetStreamFixture struct {
	js       jetstream.JetStream
	stream   jetstream.Stream
	consumer jetstream.Consumer
	subject  string
}

func newWorkerJetStreamFixture(t *testing.T) *workerJetStreamFixture {
	return newWorkerJetStreamFixtureWithMaxAckPending(t, 1)
}

func newWorkerJetStreamFixtureWithMaxAckPending(
	t *testing.T,
	maxAckPending int,
) *workerJetStreamFixture {
	t.Helper()

	_ = godotenv.Load("../../.env")

	natsURL := os.Getenv("NATS_TEST_URL")
	if natsURL == "" {
		t.Skip("NATS_TEST_URL is not configured")
	}

	conn, err := infra.NewNats(natsURL)
	if err != nil {
		t.Fatalf("connect to NATS: %v", err)
	}

	t.Cleanup(conn.Close)

	js, err := jetstream.New(conn)
	if err != nil {
		t.Fatalf("create JetStream client: %v", err)
	}

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)

	streamName := "PD_WORKER_TEST_" + strings.ToUpper(suffix)
	subject := "phishing.test.worker." + suffix
	durable := "pd-worker-test-" + suffix

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	stream, err := js.CreateStream(
		ctx,
		jetstream.StreamConfig{
			Name:      streamName,
			Subjects:  []string{subject},
			Retention: jetstream.WorkQueuePolicy,
			Storage:   jetstream.MemoryStorage,
		},
	)
	if err != nil {
		t.Fatalf("create test stream: %v", err)
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(
			context.Background(),
			3*time.Second,
		)
		defer cleanupCancel()

		if err := js.DeleteStream(cleanupCtx, streamName); err != nil {
			t.Errorf("delete test stream: %v", err)
		}
	})

	consumer, err := stream.CreateConsumer(
		ctx,
		jetstream.ConsumerConfig{
			Durable:       durable,
			FilterSubject: subject,
			AckPolicy:     jetstream.AckExplicitPolicy,
			AckWait:       30 * time.Second,
			MaxAckPending: maxAckPending,
		},
	)
	if err != nil {
		t.Fatalf("create test consumer: %v", err)
	}

	return &workerJetStreamFixture{
		js:       js,
		consumer: consumer,
		stream:   stream,
		subject:  subject,
	}
}

func (f *workerJetStreamFixture) publish(
	t *testing.T,
	payload []byte,
) {
	t.Helper()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	ack, err := f.js.Publish(ctx, f.subject, payload)
	if err != nil {
		t.Fatalf("publish worker message: %v", err)
	}

	if ack == nil {
		t.Fatal("JetStream returned empty publish acknowledgment")
	}
}

func (f *workerJetStreamFixture) fetchOne(
	t *testing.T,
) jetstream.Msg {
	t.Helper()

	return f.fetchOneWithWait(t, 3*time.Second)
}

func (f *workerJetStreamFixture) fetchMany(
	t *testing.T,
	count int,
) []jetstream.Msg {
	t.Helper()

	batch, err := f.consumer.Fetch(
		count,
		jetstream.FetchMaxWait(3*time.Second),
	)
	if err != nil {
		t.Fatalf("fetch messages: %v", err)
	}

	var messages []jetstream.Msg

	for message := range batch.Messages() {
		messages = append(messages, message)
	}

	if err := batch.Error(); err != nil {
		t.Fatalf("message batch error: %v", err)
	}

	if len(messages) != count {
		t.Fatalf(
			"expected %d messages, received %d",
			count,
			len(messages),
		)
	}

	return messages
}

func (f *workerJetStreamFixture) fetchOneWithWait(
	t *testing.T,
	wait time.Duration,
) jetstream.Msg {
	t.Helper()

	batch, err := f.consumer.Fetch(
		1,
		jetstream.FetchMaxWait(wait),
	)
	if err != nil {
		t.Fatalf("fetch worker message: %v", err)
	}

	var received jetstream.Msg
	var count int

	for message := range batch.Messages() {
		received = message
		count++
	}

	if err := batch.Error(); err != nil {
		t.Fatalf("worker message batch: %v", err)
	}

	if count != 1 || received == nil {
		t.Fatalf("expected one message, received %d", count)
	}

	return received
}

func (f *workerJetStreamFixture) assertNoPendingAcks(t *testing.T) {
	t.Helper()

	ctx, cancel := context.WithTimeout(
		context.Background(),
		3*time.Second,
	)
	defer cancel()

	info, err := f.consumer.Info(ctx)
	if err != nil {
		t.Fatalf("get consumer info: %v", err)
	}

	if info.NumAckPending != 0 {
		t.Fatalf(
			"expected no pending ACKs, got %d",
			info.NumAckPending,
		)
	}
}

func (f *workerJetStreamFixture) assertTerminated(t *testing.T) {
	t.Helper()

	deadline := time.Now().Add(3 * time.Second)

	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			time.Second,
		)

		streamInfo, streamErr := f.stream.Info(ctx)
		consumerInfo, consumerErr := f.consumer.Info(ctx)

		cancel()

		if streamErr != nil {
			t.Fatalf("get stream info: %v", streamErr)
		}

		if consumerErr != nil {
			t.Fatalf("get consumer info: %v", consumerErr)
		}

		if streamInfo.State.Msgs == 0 &&
			consumerInfo.NumAckPending == 0 &&
			consumerInfo.NumPending == 0 {
			return
		}

		time.Sleep(50 * time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Second,
	)
	defer cancel()

	streamInfo, err := f.stream.Info(ctx)
	if err != nil {
		t.Fatalf("get final stream info: %v", err)
	}

	consumerInfo, err := f.consumer.Info(ctx)
	if err != nil {
		t.Fatalf("get final consumer info: %v", err)
	}

	t.Fatalf(
		"message was not terminated: stream_messages=%d ack_pending=%d pending=%d",
		streamInfo.State.Msgs,
		consumerInfo.NumAckPending,
		consumerInfo.NumPending,
	)
}

func createPendingWorkerScan(
	t *testing.T,
	db *gorm.DB,
	userID uint,
	rawURL string,
) infra.ScanModel {
	t.Helper()

	scan := infra.ScanModel{
		UserID:    userID,
		URL:       rawURL,
		Status:    services_scan.ScanStatusPending,
		RiskLevel: "unknown",
	}

	if err := db.Create(&scan).Error; err != nil {
		t.Fatalf("create pending worker scan: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Delete(&infra.ScanModel{}, scan.ID).Error; err != nil {
			t.Errorf("delete worker scan %d: %v", scan.ID, err)
		}
	})

	return scan
}

func publishScanID(
	t *testing.T,
	fixture *workerJetStreamFixture,
	scanID uint,
) {
	t.Helper()

	payload := []byte(strconv.FormatUint(uint64(scanID), 10))
	fixture.publish(t, payload)
}

func newWorkerScanService(db *gorm.DB) *services_scan.Scan {
	return services_scan.NewServices(
		db,
		nil,
		nil,
		slog.Default(),
	)
}

func loadWorkerScan(
	t *testing.T,
	db *gorm.DB,
	scanID uint,
) infra.ScanModel {
	t.Helper()

	var scan infra.ScanModel

	if err := db.
		Preload("Indicators").
		First(&scan, scanID).Error; err != nil {
		t.Fatalf("load worker scan %d: %v", scanID, err)
	}

	return scan
}

func indicatorIDs(scan infra.ScanModel) map[string]uint {
	result := make(map[string]uint, len(scan.Indicators))

	for _, indicator := range scan.Indicators {
		result[indicator.Code] = indicator.ID
	}

	return result
}

func TestConsumerProcessesValidScan(t *testing.T) {
	db := openTestDB(t)
	fixture := newWorkerJetStreamFixture(t)

	user := createOwnershipUser(
		t,
		db,
		"worker-success",
		auth.RoleUser,
	)

	rawURL := "http://192.0.2.1/login"

	scan := createPendingWorkerScan(
		t,
		db,
		user.ID,
		rawURL,
	)

	publishScanID(t, fixture, scan.ID)

	message := fixture.fetchOne(t)

	service := newWorkerScanService(db)

	workerCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	service.HandleJetStreamMessage(workerCtx, message)

	result := loadWorkerScan(t, db, scan.ID)

	if result.ProcessingToken != "" {
		t.Fatal("processing token must be cleared after completion")
	}

	if result.Status != services_scan.ScanStatusCompleted {
		t.Fatalf(
			"expected status %q, got %q",
			services_scan.ScanStatusCompleted,
			result.Status,
		)
	}

	if result.NormalizedURL != rawURL {
		t.Fatalf(
			"expected normalized URL %q, got %q",
			rawURL,
			result.NormalizedURL,
		)
	}

	// HTTP_ONLY: 20, IP_HOST: 35, SUSPICIOUS_WORD: 7.
	if result.Score != 62 {
		t.Fatalf("expected score 62, got %d", result.Score)
	}

	if result.RiskLevel != "high" {
		t.Fatalf("expected high risk, got %q", result.RiskLevel)
	}

	if result.Error != "" {
		t.Fatalf("expected empty error, got %q", result.Error)
	}

	expectedCodes := map[string]bool{
		"HTTP_ONLY":       true,
		"IP_HOST":         true,
		"SUSPICIOUS_WORD": true,
	}

	for _, indicator := range result.Indicators {
		delete(expectedCodes, indicator.Code)
	}

	if len(expectedCodes) != 0 {
		t.Fatalf("missing expected indicators: %v", expectedCodes)
	}

	fixture.assertNoPendingAcks(t)
}

func TestConsumerMarksInvalidURLAsFailed(t *testing.T) {
	db := openTestDB(t)
	fixture := newWorkerJetStreamFixture(t)

	user := createOwnershipUser(
		t,
		db,
		"worker-invalid-url",
		auth.RoleUser,
	)

	rawURL := "https://example.test/bad%zz"

	scan := createPendingWorkerScan(
		t,
		db,
		user.ID,
		rawURL,
	)

	publishScanID(t, fixture, scan.ID)

	message := fixture.fetchOne(t)

	service := newWorkerScanService(db)

	workerCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	service.HandleJetStreamMessage(workerCtx, message)

	result := loadWorkerScan(t, db, scan.ID)

	if result.ProcessingToken != "" {
		t.Fatal("processing token must be cleared after completion")
	}

	if result.Status != services_scan.ScanStatusFailed {
		t.Fatalf(
			"expected status %q, got %q",
			services_scan.ScanStatusFailed,
			result.Status,
		)
	}

	if result.Error == "" {
		t.Fatal("expected an error description for invalid URL")
	}

	if len(result.Indicators) != 0 {
		t.Fatalf(
			"expected no indicators for invalid URL, got %d",
			len(result.Indicators),
		)
	}

	fixture.assertNoPendingAcks(t)
}

func TestConsumerAcknowledgesDuplicateCompletedScan(t *testing.T) {
	db := openTestDB(t)
	fixture := newWorkerJetStreamFixture(t)

	user := createOwnershipUser(
		t,
		db,
		"worker-duplicate",
		auth.RoleUser,
	)

	rawURL := "http://192.0.2.1/login"

	scan := createPendingWorkerScan(
		t,
		db,
		user.ID,
		rawURL,
	)

	// Publish two separate messages with the same Scan ID.
	// We intentionally do not set Nats-Msg-Id here.
	publishScanID(t, fixture, scan.ID)
	publishScanID(t, fixture, scan.ID)

	service := newWorkerScanService(db)

	workerCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	// First delivery: process the Scan.
	firstMessage := fixture.fetchOne(t)
	service.HandleJetStreamMessage(workerCtx, firstMessage)

	firstResult := loadWorkerScan(t, db, scan.ID)

	if firstResult.Status != services_scan.ScanStatusCompleted {
		t.Fatalf(
			"expected completed status after first delivery, got %q",
			firstResult.Status,
		)
	}

	firstIndicatorIDs := indicatorIDs(firstResult)

	if len(firstIndicatorIDs) == 0 {
		t.Fatal("expected indicators after first processing")
	}

	// Second delivery: the completed Scan must not be processed again.
	secondMessage := fixture.fetchOne(t)
	service.HandleJetStreamMessage(workerCtx, secondMessage)

	secondResult := loadWorkerScan(t, db, scan.ID)
	secondIndicatorIDs := indicatorIDs(secondResult)

	if secondResult.Status != services_scan.ScanStatusCompleted {
		t.Fatalf(
			"expected completed status after duplicate delivery, got %q",
			secondResult.Status,
		)
	}

	if secondResult.Score != firstResult.Score {
		t.Fatalf(
			"score changed after duplicate delivery: before=%d after=%d",
			firstResult.Score,
			secondResult.Score,
		)
	}

	if secondResult.NormalizedURL != firstResult.NormalizedURL {
		t.Fatal("normalized URL changed after duplicate delivery")
	}

	if fmt.Sprint(firstIndicatorIDs) != fmt.Sprint(secondIndicatorIDs) {
		t.Fatalf(
			"indicators were unexpectedly recreated: before=%v after=%v",
			firstIndicatorIDs,
			secondIndicatorIDs,
		)
	}

	fixture.assertNoPendingAcks(t)
}

func TestConsumerRetriesMessageAfterTemporaryDatabaseError(
	t *testing.T,
) {
	db := openTestDB(t)
	fixture := newWorkerJetStreamFixture(t)

	user := createOwnershipUser(
		t,
		db,
		"worker-temporary-error",
		auth.RoleUser,
	)

	scan := createPendingWorkerScan(
		t,
		db,
		user.ID,
		"http://192.0.2.1/login",
	)

	publishScanID(t, fixture, scan.ID)

	service := newWorkerScanService(db)

	// Inject a temporary PostgreSQL query error.
	var failQueries atomic.Bool

	const callbackName = "test:temporary_scan_query_failure"

	err := db.Callback().
		Query().
		Before("gorm:query").
		Register(callbackName, func(tx *gorm.DB) {
			if failQueries.Load() &&
				tx.Statement.Table == "scan_models" {
				tx.AddError(errors.New(
					"injected temporary database error",
				))
			}
		})

	if err != nil {
		t.Fatalf("register test query callback: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Callback().Query().Remove(callbackName)
	})

	workerCtx, cancel := context.WithTimeout(
		context.Background(),
		20*time.Second,
	)
	defer cancel()

	// First delivery: PostgreSQL query fails.
	firstMessage := fixture.fetchOne(t)

	failQueries.Store(true)
	service.HandleJetStreamMessage(workerCtx, firstMessage)
	failQueries.Store(false)

	// The failed processing attempt must not complete the Scan.
	var afterFailure infra.ScanModel

	if err := db.First(&afterFailure, scan.ID).Error; err != nil {
		t.Fatalf("load scan after temporary failure: %v", err)
	}

	if afterFailure.Status != services_scan.ScanStatusPending {
		t.Fatalf(
			"expected pending status after temporary error, got %q",
			afterFailure.Status,
		)
	}

	// NAK delay is 5 seconds in the current Worker implementation.
	// Wait for JetStream to redeliver the message.
	secondMessage := fixture.fetchOneWithWait(
		t,
		8*time.Second,
	)

	metadata, err := secondMessage.Metadata()
	if err != nil {
		t.Fatalf("get redelivered message metadata: %v", err)
	}

	if metadata.NumDelivered < 2 {
		t.Fatalf(
			"expected at least 2 deliveries, got %d",
			metadata.NumDelivered,
		)
	}

	// The temporary database error is gone.
	// The second attempt should complete successfully.
	service.HandleJetStreamMessage(workerCtx, secondMessage)

	result := loadWorkerScan(t, db, scan.ID)

	if result.Status != services_scan.ScanStatusCompleted {
		t.Fatalf(
			"expected completed status after retry, got %q",
			result.Status,
		)
	}

	if result.Score != 62 {
		t.Fatalf("expected score 62, got %d", result.Score)
	}

	if result.RiskLevel != "high" {
		t.Fatalf(
			"expected high risk, got %q",
			result.RiskLevel,
		)
	}

	fixture.assertNoPendingAcks(t)
}

func TestConsumerTerminatesInvalidPayload(t *testing.T) {
	db := openTestDB(t)
	fixture := newWorkerJetStreamFixture(t)

	service := newWorkerScanService(db)

	fixture.publish(t, []byte("not-a-valid-scan-id"))

	message := fixture.fetchOne(t)

	workerCtx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	service.HandleJetStreamMessage(workerCtx, message)

	fixture.assertTerminated(t)
}

func TestConsumerTerminatesWhenScanDoesNotExist(t *testing.T) {
	db := openTestDB(t)
	fixture := newWorkerJetStreamFixture(t)

	user := createOwnershipUser(
		t,
		db,
		"worker-missing-scan",
		auth.RoleUser,
	)

	scan := createPendingWorkerScan(
		t,
		db,
		user.ID,
		"https://example.test/missing",
	)

	// Remove Scan before publishing its ID.
	if err := db.Delete(&infra.ScanModel{}, scan.ID).Error; err != nil {
		t.Fatalf("delete scan before publishing: %v", err)
	}

	publishScanID(t, fixture, scan.ID)

	message := fixture.fetchOne(t)

	service := newWorkerScanService(db)

	workerCtx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	service.HandleJetStreamMessage(workerCtx, message)

	var count int64

	if err := db.Model(&infra.ScanModel{}).
		Where("id = ?", scan.ID).
		Count(&count).Error; err != nil {
		t.Fatalf("check deleted scan: %v", err)
	}

	if count != 0 {
		t.Fatalf("expected scan to remain deleted, got count %d", count)
	}

	fixture.assertTerminated(t)
}

func TestConsumerPreventsConcurrentProcessingOfSameScan(
	t *testing.T,
) {
	db := openTestDB(t)

	fixture := newWorkerJetStreamFixtureWithMaxAckPending(t, 2)

	user := createOwnershipUser(
		t,
		db,
		"worker-concurrent",
		auth.RoleUser,
	)

	scan := createPendingWorkerScan(
		t,
		db,
		user.ID,
		"http://192.0.2.1/login",
	)

	// Two independent messages reference the same Scan.
	publishScanID(t, fixture, scan.ID)
	publishScanID(t, fixture, scan.ID)

	messages := fixture.fetchMany(t, 2)

	service := newWorkerScanService(db)

	workerCtx, cancel := context.WithTimeout(
		context.Background(),
		20*time.Second,
	)
	defer cancel()

	// Block the first worker while it inserts indicators.
	// This gives the second worker time to attempt the same claim.
	started := make(chan struct{})
	release := make(chan struct{})

	var callbackExecuted atomic.Bool

	const callbackName = "test:block_scan_indicator_insert"

	err := db.Callback().
		Create().
		Before("gorm:create").
		Register(callbackName, func(tx *gorm.DB) {
			if tx.Statement.Table != "indicator_models" {
				return
			}

			if callbackExecuted.CompareAndSwap(false, true) {
				close(started)
				<-release
			}
		})

	if err != nil {
		t.Fatalf("register create callback: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Callback().
			Create().
			Remove(callbackName); err != nil {
			t.Errorf("remove create callback: %v", err)
		}
	})

	firstDone := make(chan struct{})

	go func() {
		defer close(firstDone)

		service.HandleJetStreamMessage(
			workerCtx,
			messages[0],
		)
	}()

	// Wait until the first worker has claimed the Scan
	// and entered indicator persistence.
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		close(release)
		<-firstDone

		t.Fatal("first worker did not reach indicator insertion")
	}

	// The first worker still owns the processing claim.
	// The second worker must NAK its message.
	service.HandleJetStreamMessage(
		workerCtx,
		messages[1],
	)

	// Allow the first worker to finish.
	close(release)

	select {
	case <-firstDone:
	case <-time.After(3 * time.Second):
		t.Fatal("first worker did not finish")
	}

	// First worker must have completed the Scan.
	firstResult := loadWorkerScan(t, db, scan.ID)

	if firstResult.Status != services_scan.ScanStatusCompleted {
		t.Fatalf(
			"expected completed status, got %q",
			firstResult.Status,
		)
	}

	firstIndicatorIDs := indicatorIDs(firstResult)

	if len(firstIndicatorIDs) != 3 {
		t.Fatalf(
			"expected 3 indicators, got %d",
			len(firstIndicatorIDs),
		)
	}

	// The second message should now be redelivered after its NAK delay.
	retryMessage := fixture.fetchOneWithWait(
		t,
		8*time.Second,
	)

	metadata, err := retryMessage.Metadata()
	if err != nil {
		t.Fatalf("get retry metadata: %v", err)
	}

	if metadata.NumDelivered < 2 {
		t.Fatalf(
			"expected redelivery count >= 2, got %d",
			metadata.NumDelivered,
		)
	}

	// The Scan is already completed.
	// The second delivery must be ACKed without reprocessing.
	service.HandleJetStreamMessage(
		workerCtx,
		retryMessage,
	)

	finalResult := loadWorkerScan(t, db, scan.ID)
	finalIndicatorIDs := indicatorIDs(finalResult)

	if len(finalIndicatorIDs) != len(firstIndicatorIDs) {
		t.Fatalf(
			"indicator count changed: before=%d after=%d",
			len(firstIndicatorIDs),
			len(finalIndicatorIDs),
		)
	}

	for code, id := range firstIndicatorIDs {
		if finalIndicatorIDs[code] != id {
			t.Fatalf(
				"indicator %q was recreated: before=%d after=%d",
				code,
				id,
				finalIndicatorIDs[code],
			)
		}
	}

	fixture.assertNoPendingAcks(t)
}

func TestConsumerRecoversStaleProcessingScan(t *testing.T) {
	db := openTestDB(t)
	fixture := newWorkerJetStreamFixture(t)

	user := createOwnershipUser(
		t,
		db,
		"worker-stale-recovery",
		auth.RoleUser,
	)

	scan := createPendingWorkerScan(
		t,
		db,
		user.ID,
		"http://192.0.2.1/login",
	)

	// Simulate an application crash after the Scan was claimed.
	staleTime := time.Now().UTC().Add(-3 * time.Minute)

	if err := db.Model(&infra.ScanModel{}).
		Where("id = ?", scan.ID).
		Updates(map[string]interface{}{
			"status":     services_scan.ScanStatusProcessing,
			"updated_at": staleTime,
		}).Error; err != nil {
		t.Fatalf("simulate stale processing Scan: %v", err)
	}

	var stale infra.ScanModel

	if err := db.First(&stale, scan.ID).Error; err != nil {
		t.Fatalf("load stale Scan: %v", err)
	}

	if stale.Status != services_scan.ScanStatusProcessing {
		t.Fatalf("expected processing, got %q", stale.Status)
	}

	if !stale.UpdatedAt.Before(time.Now().Add(-2 * time.Minute)) {
		t.Fatal("Scan is not stale")
	}

	publishScanID(t, fixture, scan.ID)

	message := fixture.fetchOne(t)

	service := newWorkerScanService(db)

	workerCtx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	service.HandleJetStreamMessage(workerCtx, message)

	result := loadWorkerScan(t, db, scan.ID)

	if result.ProcessingToken != "" {
		t.Fatal("processing token must be cleared after completion")
	}

	if result.Status != services_scan.ScanStatusCompleted {
		t.Fatalf(
			"expected completed after recovery, got %q",
			result.Status,
		)
	}

	if result.Score != 62 {
		t.Fatalf("expected score 62, got %d", result.Score)
	}

	if result.RiskLevel != "high" {
		t.Fatalf("expected high risk, got %q", result.RiskLevel)
	}

	if len(result.Indicators) != 3 {
		t.Fatalf(
			"expected 3 indicators, got %d",
			len(result.Indicators),
		)
	}

	fixture.assertNoPendingAcks(t)
}

func TestConsumerFencingTokenRejectsStaleWorker(t *testing.T) {
	db := openTestDB(t)
	fixture := newWorkerJetStreamFixture(t)

	user := createOwnershipUser(
		t,
		db,
		"worker-fencing",
		auth.RoleUser,
	)

	scan := createPendingWorkerScan(
		t,
		db,
		user.ID,
		"http://192.0.2.1/login",
	)

	publishScanID(t, fixture, scan.ID)

	message := fixture.fetchOne(t)
	service := newWorkerScanService(db)

	workerCtx, cancel := context.WithTimeout(
		context.Background(),
		15*time.Second,
	)
	defer cancel()

	// Block Worker A while it is inserting indicators.
	started := make(chan struct{})
	release := make(chan struct{})

	var releaseOnce sync.Once
	releaseWorker := func() {
		releaseOnce.Do(func() {
			close(release)
		})
	}
	t.Cleanup(releaseWorker)

	var callbackExecuted atomic.Bool

	const callbackName = "test:block_stale_worker_insert"

	if err := db.Callback().
		Create().
		Before("gorm:create").
		Register(callbackName, func(tx *gorm.DB) {
			if tx.Statement.Table != "indicator_models" {
				return
			}

			if callbackExecuted.CompareAndSwap(false, true) {
				close(started)
				<-release
			}
		}); err != nil {
		t.Fatalf("register test callback: %v", err)
	}

	t.Cleanup(func() {
		_ = db.Callback().Create().Remove(callbackName)
	})

	workerADone := make(chan struct{})

	go func() {
		defer close(workerADone)

		service.HandleJetStreamMessage(
			workerCtx,
			message,
		)
	}()

	// Wait until Worker A has claimed the Scan.
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		releaseWorker()
		<-workerADone
		t.Fatal("Worker A did not reach indicator insertion")
	}

	// Read Worker A's token.
	var claimed infra.ScanModel

	if err := db.First(&claimed, scan.ID).Error; err != nil {
		releaseWorker()
		<-workerADone
		t.Fatalf("load claimed Scan: %v", err)
	}

	if claimed.Status != services_scan.ScanStatusProcessing {
		releaseWorker()
		<-workerADone
		t.Fatalf("expected processing, got %q", claimed.Status)
	}

	tokenA := claimed.ProcessingToken

	if tokenA == "" {
		releaseWorker()
		<-workerADone
		t.Fatal("Worker A processing token is empty")
	}

	// Simulate lease expiration.
	staleTime := time.Now().UTC().Add(-3 * time.Minute)

	if err := db.Model(&infra.ScanModel{}).
		Where("id = ?", scan.ID).
		Update("updated_at", staleTime).Error; err != nil {
		releaseWorker()
		<-workerADone
		t.Fatalf("expire Worker A lease: %v", err)
	}

	// Simulate Worker B successfully claiming the expired Scan.
	tokenB := strings.Repeat("b", 32)

	if tokenA == tokenB {
		releaseWorker()
		<-workerADone
		t.Fatal("test tokens must be different")
	}

	claimB := db.Model(&infra.ScanModel{}).
		Where(
			"id = ? AND status = ? AND updated_at < ?",
			scan.ID,
			services_scan.ScanStatusProcessing,
			time.Now().UTC().Add(-2*time.Minute),
		).
		Updates(map[string]interface{}{
			"status":           services_scan.ScanStatusProcessing,
			"processing_token": tokenB,
			"updated_at":       time.Now().UTC(),
		})

	if claimB.Error != nil {
		releaseWorker()
		<-workerADone
		t.Fatalf("Worker B claim: %v", claimB.Error)
	}

	if claimB.RowsAffected != 1 {
		releaseWorker()
		<-workerADone
		t.Fatalf(
			"expected Worker B to claim one Scan, got %d",
			claimB.RowsAffected,
		)
	}

	// Worker B now owns the Scan.
	// Allow stale Worker A to continue.
	releaseWorker()

	select {
	case <-workerADone:
	case <-time.After(5 * time.Second):
		t.Fatal("Worker A did not finish")
	}

	// Worker A's transaction must have rolled back.
	var afterWorkerA infra.ScanModel

	if err := db.First(&afterWorkerA, scan.ID).Error; err != nil {
		t.Fatalf("load Scan after Worker A: %v", err)
	}

	if afterWorkerA.Status != services_scan.ScanStatusProcessing {
		t.Fatalf(
			"stale Worker A changed Scan status: %q",
			afterWorkerA.Status,
		)
	}

	if afterWorkerA.ProcessingToken != tokenB {
		t.Fatalf(
			"Worker B token was overwritten: expected %q, got %q",
			tokenB,
			afterWorkerA.ProcessingToken,
		)
	}

	var indicatorCount int64

	if err := db.Model(&infra.IndicatorModel{}).
		Where("scan_id = ?", scan.ID).
		Count(&indicatorCount).Error; err != nil {
		t.Fatalf("count indicators after Worker A: %v", err)
	}

	if indicatorCount != 0 {
		t.Fatalf(
			"Worker A's transaction should have rolled back, got %d indicators",
			indicatorCount,
		)
	}

	// Simulate Worker B successfully persisting its result.
	workerBURL := "https://worker-b.example.test"

	err := db.Transaction(func(tx *gorm.DB) error {
		indicator := infra.IndicatorModel{
			ScanID:      scan.ID,
			Code:        "WORKER_B_RESULT",
			Description: "Persisted by the current worker",
			Weight:      99,
		}

		if err := tx.Create(&indicator).Error; err != nil {
			return err
		}

		result := tx.Model(&infra.ScanModel{}).
			Where(
				"id = ? AND status = ? AND processing_token = ?",
				scan.ID,
				services_scan.ScanStatusProcessing,
				tokenB,
			).
			Updates(map[string]interface{}{
				"status":           services_scan.ScanStatusCompleted,
				"normalized_url":   workerBURL,
				"score":            99,
				"risk_level":       "critical",
				"processing_token": "",
			})

		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected != 1 {
			return errors.New("Worker B lost its processing claim")
		}

		return nil
	})

	if err != nil {
		t.Fatalf("persist Worker B result: %v", err)
	}

	final := loadWorkerScan(t, db, scan.ID)

	if final.Status != services_scan.ScanStatusCompleted {
		t.Fatalf("expected completed, got %q", final.Status)
	}

	if final.Score != 99 || final.RiskLevel != "critical" {
		t.Fatalf(
			"Worker B result was overwritten: score=%d risk=%q",
			final.Score,
			final.RiskLevel,
		)
	}

	if final.NormalizedURL != workerBURL {
		t.Fatalf(
			"expected Worker B URL %q, got %q",
			workerBURL,
			final.NormalizedURL,
		)
	}

	if final.ProcessingToken != "" {
		t.Fatal("processing token was not cleared")
	}

	if len(final.Indicators) != 1 ||
		final.Indicators[0].Code != "WORKER_B_RESULT" {
		t.Fatalf(
			"Worker B indicators were not preserved: %+v",
			final.Indicators,
		)
	}
}
