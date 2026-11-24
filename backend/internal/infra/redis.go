package infra

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

func NewRedis(
	ctx context.Context,
	redisAddr string,
) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("ping Redis: %w", err)
	}

	return client, nil
}
