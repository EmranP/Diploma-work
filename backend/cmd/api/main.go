package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/patdyyew-emran/phishing-detector/config"
	"github.com/patdyyew-emran/phishing-detector/internal/app"
)

func main() {
	logger := slog.New(
		slog.NewJSONHandler(os.Stdout, nil),
	)

	if err := run(logger); err != nil {
		logger.Error("application terminated with error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg := config.LoadConfig()

	signalCtx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	application, err := app.New(
		signalCtx,
		cfg,
		logger,
	)
	if err != nil {
		return fmt.Errorf("initialize application: %w", err)
	}

	if err := signalCtx.Err(); err != nil {
		return fmt.Errorf("application startup canceled: %w", err)
	}

	defer func() {
		if err := application.Close(); err != nil {
			logger.Error("application cleanup failed", "error", err)
		}
	}()

	addr := ":" + cfg.Port

	listenErr := make(chan error, 1)

	go func() {
		listenErr <- application.Listen(addr)
	}()

	select {
	case <-application.Ready():
		logger.Info("HTTP server started", "address", addr)

	case err := <-listenErr:
		if err != nil {
			return fmt.Errorf("HTTP server startup: %w", err)
		}

		return errors.New("HTTP server stopped before becoming ready")
	}

	select {
	case err := <-listenErr:
		if err != nil {
			return fmt.Errorf("HTTP server: %w", err)
		}

		return errors.New("HTTP server stopped unexpectedly")

	case <-signalCtx.Done():
		logger.Info("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		20*time.Second,
	)
	defer cancel()

	shutdownErr := application.Shutdown(shutdownCtx)

	serverErr := <-listenErr

	if shutdownErr != nil {
		shutdownErr = fmt.Errorf(
			"graceful shutdown: %w",
			shutdownErr,
		)
	}

	if serverErr != nil {
		serverErr = fmt.Errorf(
			"HTTP server stopped: %w",
			serverErr,
		)
	}

	if err := errors.Join(shutdownErr, serverErr); err != nil {
		return err
	}

	logger.Info("application shutdown completed")

	return nil
}
