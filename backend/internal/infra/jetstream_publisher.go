package infra

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nats-io/nats.go/jetstream"
)

type JetStreamPublisher struct {
	publisher jetstream.Publisher
}

func NewJetStreamPublisher(
	publisher jetstream.Publisher,
) (*JetStreamPublisher, error) {
	if publisher == nil {
		return nil, errors.New("JetStream publisher is nil")
	}

	return &JetStreamPublisher{
		publisher: publisher,
	}, nil
}

func (p *JetStreamPublisher) Publish(
	ctx context.Context,
	subject string,
	data []byte,
) error {
	if p == nil || p.publisher == nil {
		return errors.New("JetStream publisher is not initialized")
	}

	if ctx == nil {
		return errors.New("publish context is nil")
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	if strings.TrimSpace(subject) == "" {
		return errors.New("publish subject is empty")
	}

	if len(data) == 0 {
		return errors.New("publish payload is empty")
	}

	ack, err := p.publisher.Publish(
		ctx,
		subject,
		data,
		jetstream.WithMsgID(string(data)),
	)
	if err != nil {
		return fmt.Errorf(
			"publish JetStream message to %q: %w",
			subject,
			err,
		)
	}

	if ack == nil {
		return errors.New("JetStream returned empty publish acknowledgment")
	}

	if ack.Stream == "" || ack.Sequence == 0 {
		return errors.New("JetStream returned invalid publish acknowledgment")
	}

	return nil
}
