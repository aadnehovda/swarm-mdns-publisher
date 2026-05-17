package publisher

import (
	"testing"
)

func TestServiceFromSwarmAdvertisesIngressPort(t *testing.T) {
	service := DockerService{
		Spec: ServiceSpec{
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
		Endpoint: Endpoint{
			Ports: []PortConfig{
				{
					Protocol:      "tcp",
					TargetPort:    9443,
					PublishedPort: 9443,
					PublishMode:   "ingress",
				},
			},
		},
	}

	advertised, err := ServiceFromSwarm(service, "10.45.45.2")
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
}

func TestServiceFromSwarmIgnoresDisabledService(t *testing.T) {
	advertised, err := ServiceFromSwarm(DockerService{
		Spec: ServiceSpec{Name: "disabled"},
	}, "10.45.45.2")
	if err != nil {
		t.Fatal(err)
	}
	if len(advertised) != 0 {
		t.Fatalf("expected no advertised services, got %d", len(advertised))
	}
}

func TestServiceFromSwarmSkipsHostPublishedPort(t *testing.T) {
	service := DockerService{
		Spec: ServiceSpec{
			Name: "host_mode",
			Labels: map[string]string{
				labelEnable:      "true",
				labelHostname:    "host-mode.local",
				labelServiceType: "_http._tcp",
			},
		},
		Endpoint: Endpoint{
			Ports: []PortConfig{
				{
					Protocol:      "tcp",
					TargetPort:    8080,
					PublishedPort: 8080,
					PublishMode:   "host",
				},
			},
		},
	}

	advertised, err := ServiceFromSwarm(service, "10.45.45.2")
	if err != nil {
		t.Fatal(err)
	}
	if len(advertised) != 0 {
		t.Fatalf("expected no advertised services, got %d", len(advertised))
	}
}

func TestServiceFromSwarmRequiresHostnameForEnabledService(t *testing.T) {
	service := DockerService{
		Spec: ServiceSpec{
			Name: "missing_hostname",
			Labels: map[string]string{
				labelEnable:      "true",
				labelServiceType: "_http._tcp",
			},
		},
	}

	if _, err := ServiceFromSwarm(service, "10.45.45.2"); err == nil {
		t.Fatal("expected missing hostname error")
	}
}
