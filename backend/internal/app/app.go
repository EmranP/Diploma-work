package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/patdyyew-emran/phishing-detector/config"
	"github.com/patdyyew-emran/phishing-detector/internal/auth"
	"github.com/patdyyew-emran/phishing-detector/internal/infra"
	services_scan "github.com/patdyyew-emran/phishing-detector/internal/services"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type natsLifecycle interface {
	Drain() error
	Close()
}

type Application struct {
	http         *fiber.App
	httpReady    chan struct{}
	httpShutdown func(context.Context) error

	db    *gorm.DB
	redis *redis.Client
	nats  natsLifecycle

	consumer     jetstream.ConsumeContext
	workerCancel context.CancelFunc

	shutdownOnce sync.Once
	shutdownErr  error

	closeStorageFn   func() error
	consumerStopWait time.Duration

	logger *slog.Logger
}

func New(
	ctx context.Context,
	cfg *config.Config,
	logger *slog.Logger,
) (*Application, error) {
	if logger == nil {
		logger = slog.Default()
	}

	if ctx == nil {
		return nil, errors.New("application context is nil")
	}

	if cfg == nil {
		return nil, errors.New("application config is nil")
	}

	if err := auth.ValidateJWTSecret(cfg.JwtSecret); err != nil {
		return nil, fmt.Errorf("validate JWT configuration: %w", err)
	}

	db, err := infra.NewDB(cfg)
	if err != nil {
		return nil, fmt.Errorf("initialize PostgreSQL: %w", err)
	}

	redisCtx, cancelRedis := context.WithTimeout(ctx, 5*time.Second)
	redisClient, err := infra.NewRedis(redisCtx, cfg.RedisAddr)
	cancelRedis()

	if err != nil {
		closePostgres(db)
		return nil, fmt.Errorf("initialize Redis: %w", err)
	}

	nc, err := infra.NewNats(cfg.NatsURL)
	if err != nil {
		_ = redisClient.Close()
		closePostgres(db)

		return nil, fmt.Errorf("initialize NATS: %w", err)
	}

	var consumeCtx jetstream.ConsumeContext
	var workerCancel context.CancelFunc

	cleanup := func() {
		if workerCancel != nil {
			workerCancel()
		}

		if consumeCtx != nil {
			closed := consumeCtx.Closed()

			consumeCtx.Stop()

			select {
			case <-closed:
			case <-time.After(5 * time.Second):
				logger.Warn("JetStream consumer did not stop during initialization cleanup")
			}
		}

		nc.Close()
		_ = redisClient.Close()
		_ = closePostgres(db)
	}

	// Initialize JetStream, its Stream and Durable Consumer.
	js, scanConsumer, err := infra.NewScanJetStream(
		ctx,
		nc,
		services_scan.ScanSubject,
	)
	if err != nil {
		cleanup()

		return nil, fmt.Errorf(
			"initialize JetStream: %w",
			err,
		)
	}

	publisher, err := infra.NewJetStreamPublisher(js)
	if err != nil {
		cleanup()

		return nil, fmt.Errorf(
			"initialize JetStream publisher: %w",
			err,
		)
	}

	scanService := services_scan.NewServices(
		db,
		redisClient,
		publisher,
		logger,
	)

	// The worker has its own lifecycle.
	// Do not derive it from the startup context because that context
	// may be canceled immediately after application initialization.
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	workerCancel = cancelWorker

	consumeCtx, err = scanConsumer.Consume(
		func(message jetstream.Msg) {
			scanService.HandleJetStreamMessage(workerCtx, message)
		},
		jetstream.PullMaxMessages(1),
		jetstream.ConsumeErrHandler(
			func(
				_ jetstream.ConsumeContext,
				err error,
			) {
				logger.Error(
					"JetStream consumer error",
					"error", err,
				)
			},
		),
	)

	if err != nil {
		cleanup()

		return nil, fmt.Errorf(
			"start JetStream consumer: %w",
			err,
		)
	}

	if consumeCtx == nil {
		cleanup()

		return nil, errors.New(
			"start JetStream consumer: returned nil consume context",
		)
	}

	if err := nc.FlushTimeout(5 * time.Second); err != nil {
		cleanup()
		return nil, fmt.Errorf("flush NATS subscription: %w", err)
	}

	httpApp := NewRouter(Dependencies{
		DB:          db,
		ScanService: scanService,
		JWTSecret:   cfg.JwtSecret,
	})

	httpReady := make(chan struct{})

	var readyOnce sync.Once

	httpApp.Hooks().OnListen(func(fiber.ListenData) error {
		readyOnce.Do(func() {
			close(httpReady)
		})

		return nil
	})

	return &Application{
		http:         httpApp,
		httpReady:    httpReady,
		httpShutdown: httpApp.ShutdownWithContext,
		db:           db,
		redis:        redisClient,
		nats:         nc,
		consumer:     consumeCtx,
		workerCancel: workerCancel,
		logger:       logger,
	}, nil
}

func (a *Application) Listen(addr string) error {
	return a.http.Listen(addr)
}

func (a *Application) Ready() <-chan struct{} {
	return a.httpReady
}

func (a *Application) Shutdown(ctx context.Context) error {
	if a == nil {
		return nil
	}

	if ctx == nil {
		ctx = context.Background()
	}

	a.shutdownOnce.Do(func() {
		var shutdownErrors []error

		// 1. Stop accepting new HTTP requests.
		if err := a.shutdownHTTP(ctx); err != nil {
			shutdownErrors = append(
				shutdownErrors,
				fmt.Errorf("HTTP shutdown: %w", err),
			)
		}

		// 2. Drain JetStream Consumer.
		consumerErr := a.stopConsumer(ctx, true)
		if consumerErr != nil {
			shutdownErrors = append(shutdownErrors, consumerErr)
		}

		// 3. Never close dependencies while callbacks are running.
		if !a.consumerStopped() {
			shutdownErrors = append(
				shutdownErrors,
				errors.New(
					"JetStream consumer is still running; "+
						"leaving NATS, Redis and PostgreSQL open",
				),
			)

			a.shutdownErr = errors.Join(shutdownErrors...)
			return
		}

		// 4. Close NATS.
		if a.nats != nil {
			if consumerErr != nil {
				a.nats.Close()
			} else if err := a.nats.Drain(); err != nil {
				shutdownErrors = append(
					shutdownErrors,
					fmt.Errorf("NATS drain: %w", err),
				)

				a.nats.Close()
			}
		}

		// 5. Close storage after all workers have stopped.
		if err := a.closeStorage(); err != nil {
			shutdownErrors = append(shutdownErrors, err)
		}

		a.shutdownErr = errors.Join(shutdownErrors...)
	})

	return a.shutdownErr
}

func (a *Application) shutdownHTTP(ctx context.Context) error {
	if a.httpShutdown != nil {
		return a.httpShutdown(ctx)
	}

	if a.http != nil {
		return a.http.ShutdownWithContext(ctx)
	}

	return nil
}

func (a *Application) Close() error {
	if a == nil {
		return nil
	}

	a.shutdownOnce.Do(func() {
		var closeErrors []error

		ctx, cancel := context.WithTimeout(
			context.Background(),
			3*time.Second,
		)
		defer cancel()

		if err := a.stopConsumer(ctx, false); err != nil {
			closeErrors = append(closeErrors, err)
		}

		if !a.consumerStopped() {
			closeErrors = append(
				closeErrors,
				errors.New(
					"JetStream consumer is still running; "+
						"dependencies were not closed",
				),
			)

			a.shutdownErr = errors.Join(closeErrors...)
			return
		}

		if a.nats != nil {
			a.nats.Close()
		}

		if err := a.closeStorage(); err != nil {
			closeErrors = append(closeErrors, err)
		}

		a.shutdownErr = errors.Join(closeErrors...)
	})

	return a.shutdownErr
}

func (a *Application) stopConsumer(
	ctx context.Context,
	drain bool,
) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if a.consumer == nil {
		if a.workerCancel != nil {
			a.workerCancel()
		}

		return nil
	}

	closed := a.consumer.Closed()

	// Consumer might have stopped before Shutdown started.
	select {
	case <-closed:
		if a.workerCancel != nil {
			a.workerCancel()
		}

		return nil
	default:
	}

	if drain {
		a.consumer.Drain()
	} else {
		a.consumer.Stop()

		if a.workerCancel != nil {
			a.workerCancel()
		}
	}

	select {
	case <-closed:
		if a.workerCancel != nil {
			a.workerCancel()
		}

		return nil

	case <-ctx.Done():
		// Check once more before forcing a stop.
		select {
		case <-closed:
			if a.workerCancel != nil {
				a.workerCancel()
			}

			return nil
		default:
		}

		a.consumer.Stop()

		if a.workerCancel != nil {
			a.workerCancel()
		}

		waitCtx, cancel := context.WithTimeout(
			context.Background(),
			a.consumerStopWaitDuration(),
		)
		defer cancel()

		select {
		case <-closed:
			return fmt.Errorf(
				"consumer stopped after shutdown deadline: %w",
				ctx.Err(),
			)

		case <-waitCtx.Done():
			return errors.Join(
				fmt.Errorf("shutdown context: %w", ctx.Err()),
				fmt.Errorf("consumer stop wait: %w", waitCtx.Err()),
			)
		}
	}
}

func (a *Application) consumerStopped() bool {
	if a.consumer == nil {
		return true
	}

	select {
	case <-a.consumer.Closed():
		return true
	default:
		return false
	}
}

func (a *Application) consumerStopWaitDuration() time.Duration {
	if a.consumerStopWait > 0 {
		return a.consumerStopWait
	}

	return 5 * time.Second
}

func (a *Application) closeStorage() error {
	if a.closeStorageFn != nil {
		return a.closeStorageFn()
	}

	var closeErrors []error

	if a.redis != nil {
		if err := a.redis.Close(); err != nil {
			closeErrors = append(
				closeErrors,
				fmt.Errorf("close Redis: %w", err),
			)
		}
	}

	if err := closePostgres(a.db); err != nil {
		closeErrors = append(closeErrors, err)
	}

	return errors.Join(closeErrors...)
}

func closePostgres(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get PostgreSQL connection pool: %w", err)
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("close PostgreSQL: %w", err)
	}

	return nil
}
