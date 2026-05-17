package main

import (
	"context"
	"io"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"swarm-mdns-publisher/internal/publisher"
)

func main() {
	// hashicorp/mdns writes packet parse noise to the global standard logger.
	log.SetOutput(io.Discard)

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg := publisher.Config{
		DefaultAddress: getenv("MDNS_DEFAULT_ADDRESS", "10.45.45.2"),
		RefreshEvery:   5 * time.Minute,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := publisher.New(cfg, logger)
	if err != nil {
		logger.Error("initialize publisher", "err", err)
		os.Exit(1)
	}
	defer app.Close()

	if err := app.Run(ctx); err != nil && ctx.Err() == nil {
		logger.Error("publisher stopped", "err", err)
		os.Exit(1)
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
