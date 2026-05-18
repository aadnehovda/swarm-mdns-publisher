package publisher

import (
	"net"
	"testing"

	"github.com/docker/docker/api/types/swarm"
)

const localNodeID = "local-node"

func TestServiceFromSwarmAdvertisesIngressPort(t *testing.T) {
	service := enabledService("portainer_server", map[string]string{
		labelHostname:    "portainer.local",
		labelAddress:     "10.45.45.2",
		labelServiceType: "_https._tcp",
		labelServiceName: "Portainer",
		stackLabel:       "portainer",
	}, []swarm.PortConfig{
		{
			Protocol:      swarm.PortConfigProtocolTCP,
			TargetPort:    9443,
			PublishedPort: 9443,
			PublishMode:   swarm.PortConfigPublishModeIngress,
		},
	})

	advertised, err := ServiceFromSwarm(service, nil, AddressConfig{
		DefaultAddress: "10.45.45.2",
	}, nil)
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
	}, nil, AddressConfig{DefaultAddress: "10.45.45.2"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(advertised) != 0 {
		t.Fatalf("expected no advertised services, got %d", len(advertised))
	}
}

func TestServiceFromSwarmSkipsHostPublishedPortWithoutLocalTask(t *testing.T) {
	service := hostHTTPService()

	advertised, err := ServiceFromSwarm(service, nil, AddressConfig{
		DefaultAddress: "10.45.45.2",
		FallbackIP:     net.ParseIP("192.168.1.69"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(advertised) != 0 {
		t.Fatalf("expected no advertised services, got %d", len(advertised))
	}
}

func TestServiceFromSwarmAdvertisesHostPortForLocalRunningTask(t *testing.T) {
	service := hostHTTPService()
	tasks := []swarm.Task{runningTask(service.ID, localNodeID, nil)}

	advertised, err := ServiceFromSwarm(service, tasks, AddressConfig{
		DefaultAddress: "10.45.45.2",
		FallbackIP:     net.ParseIP("192.168.1.69"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(advertised) != 1 {
		t.Fatalf("expected 1 advertised service, got %d", len(advertised))
	}
	got := advertised[0]
	if got.Address.String() != "192.168.1.69" {
		t.Fatalf("host mode should use fallback IP, got %s", got.Address.String())
	}
	if got.TXT["address_source"] != "auto" {
		t.Fatalf("unexpected address source %q", got.TXT["address_source"])
	}
	if got.PublishMode != string(swarm.PortConfigPublishModeHost) {
		t.Fatalf("unexpected publish mode %q", got.PublishMode)
	}
}

func TestServiceFromSwarmUsesTaskPortStatusForDynamicHostPort(t *testing.T) {
	service := enabledService("dynamic_host", map[string]string{
		labelHostname:    "dynamic.local",
		labelServiceType: "_http._tcp",
	}, []swarm.PortConfig{
		{
			Protocol:    swarm.PortConfigProtocolTCP,
			TargetPort:  8080,
			PublishMode: swarm.PortConfigPublishModeHost,
		},
	})
	tasks := []swarm.Task{runningTask(service.ID, localNodeID, []swarm.PortConfig{
		{
			Protocol:      swarm.PortConfigProtocolTCP,
			TargetPort:    8080,
			PublishedPort: 32042,
			PublishMode:   swarm.PortConfigPublishModeHost,
		},
	})}

	advertised, err := ServiceFromSwarm(service, tasks, AddressConfig{
		FallbackIP: net.ParseIP("192.168.1.69"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(advertised) != 1 {
		t.Fatalf("expected 1 advertised service, got %d", len(advertised))
	}
	if got := advertised[0].Port; got != 32042 {
		t.Fatalf("expected task status port 32042, got %d", got)
	}
}

func TestServiceFromSwarmHostModeHonorsExplicitAddress(t *testing.T) {
	service := enabledService("host_address", map[string]string{
		labelHostname:    "host-address.local",
		labelServiceType: "_http._tcp",
		labelAddress:     "10.45.45.200",
	}, hostPort(8080, 8080))
	tasks := []swarm.Task{runningTask(service.ID, localNodeID, nil)}

	advertised, err := ServiceFromSwarm(service, tasks, AddressConfig{
		DefaultAddress: "10.45.45.2",
		FallbackIP:     net.ParseIP("192.168.1.69"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := advertised[0].Address.String(); got != "10.45.45.200" {
		t.Fatalf("unexpected address %s", got)
	}
	if got := advertised[0].TXT["address_source"]; got != labelAddress {
		t.Fatalf("unexpected address source %q", got)
	}
}

func TestServiceFromSwarmIgnoresStoppedAndNonLocalTasks(t *testing.T) {
	service := hostHTTPService()
	tasks := []swarm.Task{
		runningTask(service.ID, "other-node", nil),
		{
			ServiceID:    service.ID,
			NodeID:       localNodeID,
			DesiredState: swarm.TaskStateRunning,
			Status:       swarm.TaskStatus{State: swarm.TaskStateShutdown},
		},
	}
	localTasks := localRunningTasksByService(tasks, localNodeID)[service.ID]

	advertised, err := ServiceFromSwarm(service, localTasks, AddressConfig{
		FallbackIP: net.ParseIP("192.168.1.69"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(advertised) != 0 {
		t.Fatalf("expected no advertised services, got %d", len(advertised))
	}
}

func TestServiceFromSwarmIncludesCustomTXTLabels(t *testing.T) {
	service := enabledService("homeassistant", map[string]string{
		labelHostname:                    "homeassistant.local",
		labelServiceType:                 "_home-assistant._tcp",
		"mdns.txt.version":               "2026.5.1",
		"mdns.txt.internal_url":          "http://homeassistant.local:8123",
		"mdns.txt.requires_api_password": "True",
	}, ingressPort(8123, 8123))

	advertised, err := ServiceFromSwarm(service, nil, AddressConfig{
		FallbackIP: net.ParseIP("192.168.1.69"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	txt := advertised[0].TXT
	if txt["version"] != "2026.5.1" {
		t.Fatalf("missing custom version TXT: %#v", txt)
	}
	if txt["internal_url"] != "http://homeassistant.local:8123" {
		t.Fatalf("missing custom internal_url TXT: %#v", txt)
	}
	if txt["requires_api_password"] != "True" {
		t.Fatalf("missing custom requires_api_password TXT: %#v", txt)
	}
}

func TestServiceFromSwarmSkipsReservedAndEmptyCustomTXTKeys(t *testing.T) {
	service := enabledService("reserved_txt", map[string]string{
		labelHostname:          "reserved.local",
		labelServiceType:       "_http._tcp",
		"mdns.txt.service":     "override",
		"mdns.txt.":            "empty",
		"mdns.txt.description": "kept",
	}, ingressPort(80, 8080))

	advertised, err := ServiceFromSwarm(service, nil, AddressConfig{
		FallbackIP: net.ParseIP("192.168.1.69"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	txt := advertised[0].TXT
	if txt["service"] != "reserved_txt" {
		t.Fatalf("reserved service key was overwritten: %#v", txt)
	}
	if _, exists := txt[""]; exists {
		t.Fatalf("empty TXT key should be skipped: %#v", txt)
	}
	if txt["description"] != "kept" {
		t.Fatalf("non-reserved custom TXT key missing: %#v", txt)
	}
}

func TestServicesFromSwarmDeduplicatesHostRecords(t *testing.T) {
	service := hostHTTPService()
	tasks := []swarm.Task{
		runningTask(service.ID, localNodeID, nil),
		runningTask(service.ID, localNodeID, nil),
	}

	advertised, err := ServicesFromSwarm([]swarm.Service{service}, tasks, localNodeID, AddressConfig{
		FallbackIP: net.ParseIP("192.168.1.69"),
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(advertised) != 1 {
		t.Fatalf("expected 1 deduplicated service, got %d", len(advertised))
	}
}

func TestServiceFromSwarmRequiresHostnameForEnabledService(t *testing.T) {
	service := enabledService("missing_hostname", map[string]string{
		labelServiceType: "_http._tcp",
	}, nil)

	if _, err := ServiceFromSwarm(service, nil, AddressConfig{DefaultAddress: "10.45.45.2"}, nil); err == nil {
		t.Fatal("expected missing hostname error")
	}
}

func TestServiceFromSwarmUsesDefaultAddressWhenServiceAddressMissing(t *testing.T) {
	service := enabledHTTPService()

	advertised, err := ServiceFromSwarm(service, nil, AddressConfig{
		DefaultAddress: "10.45.45.2",
		FallbackIP:     net.ParseIP("192.168.1.69"),
	}, nil)
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

	advertised, err := ServiceFromSwarm(service, nil, AddressConfig{
		FallbackIP: net.ParseIP("192.168.1.69"),
	}, nil)
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

	if _, err := ServiceFromSwarm(service, nil, AddressConfig{}, nil); err == nil {
		t.Fatal("expected missing address error")
	}
}

func enabledHTTPService() swarm.Service {
	return enabledService("web", map[string]string{
		labelHostname:    "web.local",
		labelServiceType: "_http._tcp",
	}, ingressPort(80, 8080))
}

func hostHTTPService() swarm.Service {
	return enabledService("host_mode", map[string]string{
		labelHostname:    "host-mode.local",
		labelServiceType: "_http._tcp",
	}, hostPort(8080, 8080))
}

func enabledService(name string, labels map[string]string, ports []swarm.PortConfig) swarm.Service {
	allLabels := map[string]string{labelEnable: "true"}
	for key, value := range labels {
		allLabels[key] = value
	}
	return swarm.Service{
		ID: name + "-id",
		Spec: swarm.ServiceSpec{
			Annotations: swarm.Annotations{
				Name:   name,
				Labels: allLabels,
			},
		},
		Endpoint: swarm.Endpoint{Ports: ports},
	}
}

func ingressPort(target, published uint32) []swarm.PortConfig {
	return []swarm.PortConfig{{
		Protocol:      swarm.PortConfigProtocolTCP,
		TargetPort:    target,
		PublishedPort: published,
		PublishMode:   swarm.PortConfigPublishModeIngress,
	}}
}

func hostPort(target, published uint32) []swarm.PortConfig {
	return []swarm.PortConfig{{
		Protocol:      swarm.PortConfigProtocolTCP,
		TargetPort:    target,
		PublishedPort: published,
		PublishMode:   swarm.PortConfigPublishModeHost,
	}}
}

func runningTask(serviceID, nodeID string, ports []swarm.PortConfig) swarm.Task {
	return swarm.Task{
		ServiceID:    serviceID,
		NodeID:       nodeID,
		DesiredState: swarm.TaskStateRunning,
		Status: swarm.TaskStatus{
			State:      swarm.TaskStateRunning,
			PortStatus: swarm.PortStatus{Ports: ports},
		},
	}
}
