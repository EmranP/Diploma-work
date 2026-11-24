package integration_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/patdyyew-emran/phishing-detector/internal/infra"
)

func TestNewScanJetStream(t *testing.T) {
	if err := godotenv.Load("../../.env"); err != nil {
		t.Fatalf("load project .env: %v", err)
	}

	natsURL := os.Getenv("NATS_TEST_URL")
	if natsURL == "" {
		t.Skip("NATS_TEST_URL is not configured")
	}

	fmt.Println(natsURL)

	conn, err := infra.NewNats(natsURL)
	if err != nil {
		t.Fatalf("connect to NATS: %v", err)
	}
	t.Cleanup(conn.Close)

	ctx, cancel := context.WithTimeout(
		context.Background(),
		10*time.Second,
	)
	defer cancel()

	js, consumer, err := infra.NewScanJetStream(
		ctx,
		conn,
		"phishing.scan.request",
	)
	if err != nil {
		t.Fatalf("initialize JetStream: %v", err)
	}

	if js == nil {
		t.Fatal("JetStream client is nil")
	}

	if consumer == nil {
		t.Fatal("JetStream consumer is nil")
	}

	stream, err := js.Stream(ctx, infra.ScanStreamName)
	if err != nil {
		t.Fatalf("get scan stream: %v", err)
	}

	streamInfo, err := stream.Info(ctx)
	if err != nil {
		t.Fatalf("get stream info: %v", err)
	}

	if streamInfo.Config.Name != infra.ScanStreamName {
		t.Fatalf(
			"expected stream %q, got %q",
			infra.ScanStreamName,
			streamInfo.Config.Name,
		)
	}

	consumerInfo, err := consumer.Info(ctx)
	if err != nil {
		t.Fatalf("get consumer info: %v", err)
	}

	if consumerInfo.Config.Durable != infra.ScanConsumerName {
		t.Fatalf(
			"expected durable consumer %q, got %q",
			infra.ScanConsumerName,
			consumerInfo.Config.Durable,
		)
	}
}
