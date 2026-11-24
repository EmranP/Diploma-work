package integration_test

import (
	"bytes"
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/patdyyew-emran/phishing-detector/internal/infra"
)

func TestJetStreamPublisherPublish(t *testing.T) {
	// Load test environment.
	if err := godotenv.Load("../../.env"); err != nil {
		t.Fatalf("load backend .env: %v", err)
	}

	natsURL := os.Getenv("NATS_TEST_URL")
	if natsURL == "" {
		t.Skip("NATS_TEST_URL is not configured")
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	// Connect to NATS.
	conn, err := infra.NewNats(natsURL)
	if err != nil {
		t.Fatalf("connect to NATS: %v", err)
	}
	t.Cleanup(conn.Close)

	// Initialize JetStream.
	js, err := jetstream.New(conn)
	if err != nil {
		t.Fatalf("create JetStream client: %v", err)
	}

	// Create unique names to isolate this test.
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)

	streamName := "PD_PUBLISHER_TEST_" + suffix
	subject := "phishing.test.publisher." + suffix
	durable := "publisher-test-" + suffix

	// Create an isolated temporary stream.
	stream, err := js.CreateStream(
		ctx,
		jetstream.StreamConfig{
			Name:       streamName,
			Subjects:   []string{subject},
			Retention:  jetstream.WorkQueuePolicy,
			Storage:    jetstream.MemoryStorage,
			Duplicates: 2 * time.Minute,
		},
	)
	if err != nil {
		t.Fatalf("create test stream: %v", err)
	}

	// Delete the temporary stream after the test.
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

	// Create a durable pull consumer.
	consumer, err := stream.CreateConsumer(
		ctx,
		jetstream.ConsumerConfig{
			Durable:       durable,
			FilterSubject: subject,
			AckPolicy:     jetstream.AckExplicitPolicy,
			AckWait:       30 * time.Second,
		},
	)
	if err != nil {
		t.Fatalf("create test consumer: %v", err)
	}

	// Initialize our application publisher.
	publisher, err := infra.NewJetStreamPublisher(js)
	if err != nil {
		t.Fatalf("create JetStream publisher: %v", err)
	}

	payload := []byte(`{"scan_id":123}`)

	// First publication.
	if err := publisher.Publish(ctx, subject, payload); err != nil {
		t.Fatalf("first publish: %v", err)
	}

	// Publish the same payload again.
	// Current publisher uses the payload as Nats-Msg-Id.
	if err := publisher.Publish(ctx, subject, payload); err != nil {
		t.Fatalf("second publish: %v", err)
	}

	// Verify that JetStream stored only one message.
	streamInfo, err := stream.Info(ctx)
	if err != nil {
		t.Fatalf("get stream info: %v", err)
	}

	if got := streamInfo.State.Msgs; got != 1 {
		t.Fatalf("expected 1 stored message after deduplication, got %d", got)
	}

	// Fetch the published message.
	batch, err := consumer.Fetch(
		1,
		jetstream.FetchMaxWait(2*time.Second),
	)
	if err != nil {
		t.Fatalf("fetch message: %v", err)
	}

	var received jetstream.Msg
	var receivedCount int

	for msg := range batch.Messages() {
		received = msg
		receivedCount++
	}

	if err := batch.Error(); err != nil {
		t.Fatalf("message batch error: %v", err)
	}

	if receivedCount != 1 {
		t.Fatalf("expected 1 received message, got %d", receivedCount)
	}

	if received == nil {
		t.Fatal("received message is nil")
	}

	// Verify subject.
	if received.Subject() != subject {
		t.Fatalf(
			"expected subject %q, got %q",
			subject,
			received.Subject(),
		)
	}

	// Verify payload.
	if !bytes.Equal(received.Data(), payload) {
		t.Fatalf(
			"unexpected payload: expected %s, got %s",
			payload,
			received.Data(),
		)
	}

	// Verify successful acknowledgement.
	if err := received.DoubleAck(ctx); err != nil {
		t.Fatalf("double ack message: %v", err)
	}
}
