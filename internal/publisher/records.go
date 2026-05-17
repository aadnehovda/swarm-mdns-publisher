package publisher

import (
	"fmt"
	"net"
	"sort"
	"strings"
)

const (
	labelEnable      = "mdns.enable"
	labelHostname    = "mdns.hostname"
	labelAddress     = "mdns.address"
	labelServiceType = "mdns.service.type"
	labelServiceName = "mdns.service.name"
	stackLabel       = "com.docker.stack.namespace"
)

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

func ServicesFromSwarm(services []DockerService, defaultAddress string) ([]AdvertisedService, error) {
	var advertised []AdvertisedService
	for _, service := range services {
		generated, err := ServiceFromSwarm(service, defaultAddress)
		if err != nil {
			return nil, err
		}
		advertised = append(advertised, generated...)
	}
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

func ServiceFromSwarm(service DockerService, defaultAddress string) ([]AdvertisedService, error) {
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

	addressText := labels[labelAddress]
	if addressText == "" {
		addressText = defaultAddress
	}
	address := net.ParseIP(addressText)
	if address == nil {
		return nil, fmt.Errorf("service %q has invalid mDNS address %q", service.Spec.Name, addressText)
	}

	instanceName := labels[labelServiceName]
	if instanceName == "" {
		instanceName = service.Spec.Name
	}

	var advertised []AdvertisedService
	for _, port := range service.Endpoint.Ports {
		if port.PublishMode != "ingress" {
			continue
		}
		if port.PublishedPort == 0 {
			continue
		}
		if port.Protocol != "tcp" && port.Protocol != "udp" {
			continue
		}

		stack := labels[stackLabel]
		txt := map[string]string{
			"service":        service.Spec.Name,
			"protocol":       port.Protocol,
			"target_port":    fmt.Sprint(port.TargetPort),
			"published_port": fmt.Sprint(port.PublishedPort),
			"publish_mode":   port.PublishMode,
		}
		if stack != "" {
			txt["stack"] = stack
		}

		advertised = append(advertised, AdvertisedService{
			ServiceName:  service.Spec.Name,
			Stack:        stack,
			Hostname:     hostname,
			InstanceName: instanceName,
			ServiceType:  serviceType,
			Address:      address,
			Port:         port.PublishedPort,
			TargetPort:   port.TargetPort,
			Protocol:     port.Protocol,
			PublishMode:  port.PublishMode,
			TXT:          txt,
		})
	}

	return advertised, nil
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
