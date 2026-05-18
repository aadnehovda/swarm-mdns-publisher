package publisher

import (
	"fmt"
	"log/slog"
	"net"
	"sort"
	"strings"

	"github.com/docker/docker/api/types/swarm"
)

const (
	labelEnable      = "mdns.enable"
	labelHostname    = "mdns.hostname"
	labelAddress     = "mdns.address"
	labelServiceType = "mdns.service.type"
	labelServiceName = "mdns.service.name"
	labelTXTPrefix   = "mdns.txt."
	stackLabel       = "com.docker.stack.namespace"
)

var reservedTXTKeys = map[string]struct{}{
	"service":        {},
	"stack":          {},
	"protocol":       {},
	"target_port":    {},
	"published_port": {},
	"publish_mode":   {},
	"address_source": {},
}

type AdvertisedService struct {
	ServiceName  string
	Stack        string
	Hostname     string
	InstanceName string
	ServiceType  string
	Address      net.IP
	Port         uint32
	TargetPort   uint32
	Protocol     string
	PublishMode  string
	TXT          map[string]string
}

func ServicesFromSwarm(services []swarm.Service, tasks []swarm.Task, localNodeID string, addresses AddressConfig, logger *slog.Logger) ([]AdvertisedService, error) {
	var advertised []AdvertisedService
	tasksByService := localRunningTasksByService(tasks, localNodeID)
	for _, service := range services {
		generated, err := ServiceFromSwarm(service, tasksByService[service.ID], addresses, logger)
		if err != nil {
			return nil, err
		}
		advertised = append(advertised, generated...)
	}
	advertised = dedupeAdvertisedServices(advertised)
	sort.Slice(advertised, func(i, j int) bool {
		if advertised[i].Hostname == advertised[j].Hostname {
			if advertised[i].ServiceType == advertised[j].ServiceType {
				return advertised[i].Port < advertised[j].Port
			}
			return advertised[i].ServiceType < advertised[j].ServiceType
		}
		return advertised[i].Hostname < advertised[j].Hostname
	})
	return advertised, nil
}

func ServiceFromSwarm(service swarm.Service, localTasks []swarm.Task, addresses AddressConfig, logger *slog.Logger) ([]AdvertisedService, error) {
	labels := service.Spec.Labels
	if strings.ToLower(labels[labelEnable]) != "true" {
		return nil, nil
	}

	hostname := normalizeLocalName(labels[labelHostname])
	if hostname == "" {
		return nil, fmt.Errorf("service %q has mdns.enable=true but no %s label", service.Spec.Name, labelHostname)
	}

	serviceType := normalizeServiceType(labels[labelServiceType])
	if serviceType == "" {
		return nil, nil
	}

	ingressAddress, ingressAddressSource, err := addresses.Resolve(labels)
	if err != nil {
		return nil, fmt.Errorf("service %q: %w", service.Spec.Name, err)
	}
	hostAddress, hostAddressSource, err := addresses.ResolveHost(labels)
	if err != nil {
		hostAddress = nil
	}

	instanceName := labels[labelServiceName]
	if instanceName == "" {
		instanceName = service.Spec.Name
	}

	var advertised []AdvertisedService
	for _, port := range service.Endpoint.Ports {
		if port.PublishMode != swarm.PortConfigPublishModeIngress || !advertisableProtocol(port.Protocol) || port.PublishedPort == 0 {
			continue
		}
		advertised = append(advertised, advertisedServiceForPort(service, port, ingressAddress, ingressAddressSource, instanceName, serviceType, logger))
	}

	if hostAddress == nil {
		return advertised, nil
	}
	for _, task := range localTasks {
		for _, port := range hostPortsForTask(service, task) {
			if !advertisableProtocol(port.Protocol) || port.PublishedPort == 0 {
				continue
			}
			advertised = append(advertised, advertisedServiceForPort(service, port, hostAddress, hostAddressSource, instanceName, serviceType, logger))
		}
	}

	return advertised, nil
}

func advertisedServiceForPort(service swarm.Service, port swarm.PortConfig, address net.IP, addressSource, instanceName, serviceType string, logger *slog.Logger) AdvertisedService {
	labels := service.Spec.Labels
	stack := labels[stackLabel]
	txt := map[string]string{
		"service":        service.Spec.Name,
		"protocol":       string(port.Protocol),
		"target_port":    fmt.Sprint(port.TargetPort),
		"published_port": fmt.Sprint(port.PublishedPort),
		"publish_mode":   string(port.PublishMode),
		"address_source": addressSource,
	}
	if stack != "" {
		txt["stack"] = stack
	}
	addCustomTXT(txt, service, logger)

	return AdvertisedService{
		ServiceName:  service.Spec.Name,
		Stack:        stack,
		Hostname:     normalizeLocalName(labels[labelHostname]),
		InstanceName: instanceName,
		ServiceType:  serviceType,
		Address:      address,
		Port:         port.PublishedPort,
		TargetPort:   port.TargetPort,
		Protocol:     string(port.Protocol),
		PublishMode:  string(port.PublishMode),
		TXT:          txt,
	}
}

func addCustomTXT(txt map[string]string, service swarm.Service, logger *slog.Logger) {
	for label, value := range service.Spec.Labels {
		if !strings.HasPrefix(label, labelTXTPrefix) {
			continue
		}
		key := strings.TrimSpace(strings.TrimPrefix(label, labelTXTPrefix))
		if key == "" {
			logSkippedTXT(logger, service.Spec.Name, label, "empty key")
			continue
		}
		if _, reserved := reservedTXTKeys[key]; reserved {
			logSkippedTXT(logger, service.Spec.Name, label, "reserved key")
			continue
		}
		txt[key] = value
	}
}

func logSkippedTXT(logger *slog.Logger, serviceName, label, reason string) {
	if logger != nil {
		logger.Warn("skipping custom TXT label", "service", serviceName, "label", label, "reason", reason)
	}
}

func localRunningTasksByService(tasks []swarm.Task, localNodeID string) map[string][]swarm.Task {
	byService := make(map[string][]swarm.Task)
	for _, task := range tasks {
		if task.NodeID != localNodeID {
			continue
		}
		if task.DesiredState != swarm.TaskStateRunning || task.Status.State != swarm.TaskStateRunning {
			continue
		}
		byService[task.ServiceID] = append(byService[task.ServiceID], task)
	}
	return byService
}

func hostPortsForTask(service swarm.Service, task swarm.Task) []swarm.PortConfig {
	if len(task.Status.PortStatus.Ports) > 0 {
		return filterPublishMode(task.Status.PortStatus.Ports, swarm.PortConfigPublishModeHost)
	}
	return filterPublishMode(service.Endpoint.Ports, swarm.PortConfigPublishModeHost)
}

func filterPublishMode(ports []swarm.PortConfig, publishMode swarm.PortConfigPublishMode) []swarm.PortConfig {
	filtered := make([]swarm.PortConfig, 0, len(ports))
	for _, port := range ports {
		if port.PublishMode == publishMode {
			filtered = append(filtered, port)
		}
	}
	return filtered
}

func advertisableProtocol(protocol swarm.PortConfigProtocol) bool {
	return protocol == swarm.PortConfigProtocolTCP || protocol == swarm.PortConfigProtocolUDP
}

func dedupeAdvertisedServices(services []AdvertisedService) []AdvertisedService {
	seen := make(map[string]struct{}, len(services))
	deduped := make([]AdvertisedService, 0, len(services))
	for _, service := range services {
		key := fmt.Sprintf("%s|%s|%s|%d|%d|%s", service.Hostname, service.ServiceType, service.Protocol, service.Port, service.TargetPort, service.Address)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, service)
	}
	return deduped
}

func normalizeLocalName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.TrimSuffix(value, ".")
	if !strings.HasSuffix(value, ".local") {
		value += ".local"
	}
	return value + "."
}

func normalizeServiceType(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = strings.TrimSuffix(value, ".")
	if !strings.HasSuffix(value, "._tcp") && !strings.HasSuffix(value, "._udp") {
		return ""
	}
	return value
}
