package publisher

import (
	"context"
	"log/slog"
	"net"
	"strings"
	"sync"

	"github.com/brutella/dnssd"
)

type DynamicResponder struct {
	logger    *slog.Logger
	responder dnssd.Responder

	mu      sync.Mutex
	handles map[string]dnssd.ServiceHandle
}

func NewDynamicResponder(logger *slog.Logger) (*DynamicResponder, error) {
	responder, err := dnssd.NewResponder()
	if err != nil {
		return nil, err
	}

	return &DynamicResponder{
		logger:    logger,
		responder: responder,
		handles:   make(map[string]dnssd.ServiceHandle),
	}, nil
}

func (r *DynamicResponder) Replace(advertised []AdvertisedService) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, handle := range r.handles {
		r.responder.Remove(handle)
	}
	r.handles = make(map[string]dnssd.ServiceHandle, len(advertised))

	for _, service := range advertised {
		dnssdService, err := dnssd.NewService(dnssd.Config{
			Name:   service.InstanceName,
			Type:   service.ServiceType,
			Domain: "local",
			Host:   dnssdHost(service.Hostname),
			Text:   service.TXT,
			IPs:    []net.IP{service.Address},
			Port:   int(service.Port),
		})
		if err != nil {
			return err
		}

		handle, err := r.responder.Add(dnssdService)
		if err != nil {
			return err
		}

		key := service.InstanceName + "|" + service.ServiceType + "|" + service.Hostname
		r.handles[key] = handle
	}

	r.logger.Info("updated mDNS records", "count", len(r.handles))
	for _, service := range advertised {
		r.logger.Info("advertising service",
			"docker_service", service.ServiceName,
			"hostname", service.Hostname,
			"type", service.ServiceType,
			"instance", service.InstanceName,
			"address", service.Address.String(),
			"port", service.Port,
			"protocol", service.Protocol,
		)
	}

	return nil
}

func (r *DynamicResponder) Respond(ctx context.Context) error {
	return r.responder.Respond(ctx)
}

func dnssdHost(hostname string) string {
	hostname = strings.TrimSuffix(hostname, ".")
	hostname = strings.TrimSuffix(hostname, ".local")
	return hostname
}
