package publisher

import (
	"context"
	"log/slog"
	"time"
)

type Config struct {
	DefaultAddress string
	RefreshEvery   time.Duration
}

type Publisher struct {
	cfg    Config
	logger *slog.Logger
	docker *DockerClient
	mdns   *DynamicResponder
}

func New(cfg Config, logger *slog.Logger) (*Publisher, error) {
	if cfg.RefreshEvery == 0 {
		cfg.RefreshEvery = 5 * time.Minute
	}

	dockerClient, err := NewDockerClient("")
	if err != nil {
		return nil, err
	}

	mdnsResponder, err := NewDynamicResponder(logger)
	if err != nil {
		return nil, err
	}

	return &Publisher{
		cfg:    cfg,
		logger: logger,
		docker: dockerClient,
		mdns:   mdnsResponder,
	}, nil
}

func (p *Publisher) Close() {
	if p.docker != nil {
		if err := p.docker.Close(); err != nil {
			p.logger.Error("closing Docker client failed", "err", err)
		}
	}
}

func (p *Publisher) Run(ctx context.Context) error {
	responderErrs := make(chan error, 1)
	go func() {
		responderErrs <- p.mdns.Respond(ctx)
	}()

	if err := p.refresh(ctx); err != nil {
		p.logger.Error("initial Docker service refresh failed", "err", err)
	}

	ticker := time.NewTicker(p.cfg.RefreshEvery)
	defer ticker.Stop()

	eventMessages, eventErrs := p.docker.WatchServiceEvents(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-responderErrs:
			if err != nil && ctx.Err() == nil {
				return err
			}
			return ctx.Err()
		case <-ticker.C:
			if err := p.refresh(ctx); err != nil {
				p.logger.Error("periodic Docker service refresh failed", "err", err)
			}
		case event := <-eventMessages:
			p.logger.Info("Docker service event", "action", event.Action, "actor", event.Actor.ID)
			if err := p.refresh(ctx); err != nil {
				p.logger.Error("event Docker service refresh failed", "err", err)
			}
		case err := <-eventErrs:
			if err != nil && ctx.Err() == nil {
				p.logger.Error("Docker event stream failed", "err", err)
				time.Sleep(5 * time.Second)
				eventMessages, eventErrs = p.docker.WatchServiceEvents(ctx)
			}
		}
	}
}

func (p *Publisher) refresh(ctx context.Context) error {
	services, err := p.docker.ListServices(ctx)
	if err != nil {
		return err
	}

	advertised, err := ServicesFromSwarm(services, p.cfg.DefaultAddress)
	if err != nil {
		return err
	}
	return p.mdns.Replace(advertised)
}
