package infra

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	ScanStreamName   = "PHISHING_SCANS"
	ScanConsumerName = "phishing-scan-worker"
)

func NewNats(natsURL string) (*nats.Conn, error) {
	conn, err := nats.Connect(
		natsURL,
		nats.Name("phishing-detector-monolith"),
		nats.Timeout(5*time.Second),
		nats.DrainTimeout(15*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}

	return conn, nil
}

func NewScanJetStream(
	ctx context.Context,
	conn *nats.Conn,
	subject string,
) (jetstream.JetStream, jetstream.Consumer, error) {
	if conn == nil {
		return nil, nil, errors.New("NATS connection is nil")
	}

	if ctx == nil {
		return nil, nil, errors.New("JetStream context is nil")
	}

	if strings.TrimSpace(subject) == "" {
		return nil, nil, errors.New("JetStream subject is empty")
	}

	js, err := jetstream.New(conn)
	if err != nil {
		return nil, nil, fmt.Errorf("create JetStream client: %w", err)
	}

	stream, err := js.CreateOrUpdateStream(
		ctx,
		jetstream.StreamConfig{
			Name:        ScanStreamName,
			Description: "Persistent phishing URL scan jobs",
			Subjects:    []string{subject},

			Retention: jetstream.WorkQueuePolicy,
			Storage:   jetstream.FileStorage,
			Discard:   jetstream.DiscardNew,

			MaxAge:   7 * 24 * time.Hour,
			MaxMsgs:  100_000,
			MaxBytes: 128 * 1024 * 1024,
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create scan stream: %w", err)
	}

	consumer, err := stream.CreateOrUpdateConsumer(
		ctx,
		jetstream.ConsumerConfig{
			Durable:       ScanConsumerName,
			FilterSubject: subject,

			AckPolicy: jetstream.AckExplicitPolicy,
			AckWait:   30 * time.Second,

			MaxAckPending: 2,
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create scan consumer: %w", err)
	}

	return js, consumer, nil
}
