package publisher

import (
	"net"
	"testing"

	"github.com/docker/docker/api/types/swarm"
)

func TestServiceFromSwarmAdvertisesIngressPort(t *testing.T) {
	service := swarm.Service{
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Name: "portainer_server",
				Labels: map[string]string{
					labelEnable:      "true",
					labelHostname:    "portainer.local",
					labelAddress:     "10.45.45.2",
					labelServiceType: "_https._tcp",
					labelServiceName: "Portainer",
					stackLabel:       "portainer",
				},
			},
		},
		Endpoint: swarm.Endpoint{
			Ports: []swarm.PortConfig{
				{
					Protocol:      swarm.PortConfigProtocolTCP,
					TargetPort:    9443,
					PublishedPort: 9443,
					PublishMode:   swarm.PortConfigPublishModeIngress,
				},
			},
		},
	}

	advertised, err := ServiceFromSwarm(service, AddressConfig{
		DefaultAddress: "10.45.45.2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(advertised) != 1 {
		t.Fatalf("expected 1 advertised service, got %d", len(advertised))
	}
	got := advertised[0]
	if got.Hostname != "portainer.local." {
		t.Fatalf("unexpected hostname %q", got.Hostname)
	}
	if got.ServiceType != "_https._tcp" {
		t.Fatalf("unexpected service type %q", got.ServiceType)
	}
	if got.Port != 9443 {
		t.Fatalf("unexpected port %d", got.Port)
	}
	if got.Address.String() != "10.45.45.2" {
		t.Fatalf("unexpected address %s", got.Address.String())
	}
	if got.TXT["address_source"] != labelAddress {
		t.Fatalf("unexpected address source %q", got.TXT["address_source"])
	}
}

func TestServiceFromSwarmIgnoresDisabledService(t *testing.T) {
	advertised, err := ServiceFromSwarm(swarm.Service{
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{Name: "disabled"},
		},
	}, AddressConfig{DefaultAddress: "10.45.45.2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(advertised) != 0 {
		t.Fatalf("expected no advertised services, got %d", len(advertised))
	}
}

func TestServiceFromSwarmSkipsHostPublishedPort(t *testing.T) {
	service := swarm.Service{
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Name: "host_mode",
				Labels: map[string]string{
					labelEnable:      "true",
					labelHostname:    "host-mode.local",
					labelServiceType: "_http._tcp",
				},
			},
		},
		Endpoint: swarm.Endpoint{
			Ports: []swarm.PortConfig{
				{
					Protocol:      swarm.PortConfigProtocolTCP,
					TargetPort:    8080,
					PublishedPort: 8080,
					PublishMode:   swarm.PortConfigPublishModeHost,
				},
			},
		},
	}

	advertised, err := ServiceFromSwarm(service, AddressConfig{
		DefaultAddress: "10.45.45.2",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(advertised) != 0 {
		t.Fatalf("expected no advertised services, got %d", len(advertised))
	}
}

func TestServiceFromSwarmRequiresHostnameForEnabledService(t *testing.T) {
	service := swarm.Service{
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Name: "missing_hostname",
				Labels: map[string]string{
					labelEnable:      "true",
					labelServiceType: "_http._tcp",
				},
			},
		},
	}

	if _, err := ServiceFromSwarm(service, AddressConfig{DefaultAddress: "10.45.45.2"}); err == nil {
		t.Fatal("expected missing hostname error")
	}
}

func TestServiceFromSwarmUsesDefaultAddressWhenServiceAddressMissing(t *testing.T) {
	service := enabledHTTPService()

	advertised, err := ServiceFromSwarm(service, AddressConfig{
		DefaultAddress: "10.45.45.2",
		FallbackIP:     net.ParseIP("192.168.1.69"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := advertised[0].Address.String(); got != "10.45.45.2" {
		t.Fatalf("unexpected address %s", got)
	}
	if got := advertised[0].TXT["address_source"]; got != "MDNS_DEFAULT_ADDRESS" {
		t.Fatalf("unexpected address source %q", got)
	}
}

func TestServiceFromSwarmUsesFallbackAddressWhenConfiguredAddressesMissing(t *testing.T) {
	service := enabledHTTPService()

	advertised, err := ServiceFromSwarm(service, AddressConfig{
		FallbackIP: net.ParseIP("192.168.1.69"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := advertised[0].Address.String(); got != "192.168.1.69" {
		t.Fatalf("unexpected address %s", got)
	}
	if got := advertised[0].TXT["address_source"]; got != "auto" {
		t.Fatalf("unexpected address source %q", got)
	}
}

func TestServiceFromSwarmErrorsWhenNoAddressCanBeResolved(t *testing.T) {
	service := enabledHTTPService()

	if _, err := ServiceFromSwarm(service, AddressConfig{}); err == nil {
		t.Fatal("expected missing address error")
	}
}

func enabledHTTPService() swarm.Service {
	return swarm.Service{
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Name: "web",
				Labels: map[string]string{
					labelEnable:      "true",
					labelHostname:    "web.local",
					labelServiceType: "_http._tcp",
				},
			},
		},
		Endpoint: swarm.Endpoint{
			Ports: []swarm.PortConfig{
				{
					Protocol:      swarm.PortConfigProtocolTCP,
					TargetPort:    80,
					PublishedPort: 8080,
					PublishMode:   swarm.PortConfigPublishModeIngress,
				},
			},
		},
	}
}
