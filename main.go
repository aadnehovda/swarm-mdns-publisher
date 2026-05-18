package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"swarm-mdns-publisher/internal/publisher"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	cfg := publisher.Config{
		DefaultAddress: os.Getenv("MDNS_DEFAULT_ADDRESS"),
		ProbeAddress:   os.Getenv("MDNS_PROBE_ADDRESS"),
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
