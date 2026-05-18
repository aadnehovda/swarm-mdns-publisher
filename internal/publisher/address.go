package publisher

import (
	"fmt"
	"net"
	"strings"
)

const defaultProbeAddress = "224.0.0.251:5353"

type AddressConfig struct {
	DefaultAddress string
	FallbackIP     net.IP
}

func (c AddressConfig) Resolve(labels map[string]string) (net.IP, string, error) {
	if addressText := strings.TrimSpace(labels[labelAddress]); addressText != "" {
		return parseAdvertiseIP(addressText, labelAddress)
	}
	if addressText := strings.TrimSpace(c.DefaultAddress); addressText != "" {
		return parseAdvertiseIP(addressText, "MDNS_DEFAULT_ADDRESS")
	}
	if c.FallbackIP != nil {
		return c.FallbackIP, "auto", nil
	}
	return nil, "", fmt.Errorf("no mDNS address configured and automatic address detection failed")
}

func (c AddressConfig) ResolveHost(labels map[string]string) (net.IP, string, error) {
	if addressText := strings.TrimSpace(labels[labelAddress]); addressText != "" {
		return parseAdvertiseIP(addressText, labelAddress)
	}
	if c.FallbackIP != nil {
		return c.FallbackIP, "auto", nil
	}
	return nil, "", fmt.Errorf("no host-mode mDNS address configured and automatic address detection failed")
}

func parseAdvertiseIP(value, source string) (net.IP, string, error) {
	address := net.ParseIP(value)
	if address == nil {
		return nil, source, fmt.Errorf("%s has invalid mDNS address %q", source, value)
	}
	return address, source, nil
}

func DetectOutboundIP(probeAddress string) (net.IP, error) {
	if strings.TrimSpace(probeAddress) == "" {
		probeAddress = defaultProbeAddress
	}

	remote, err := net.ResolveUDPAddr("udp", probeAddress)
	if err != nil {
		return nil, err
	}

	conn, err := net.DialUDP("udp", nil, remote)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	local, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || local.IP == nil || local.IP.IsUnspecified() {
		return nil, fmt.Errorf("could not determine local address for %s", probeAddress)
	}
	return local.IP, nil
}
