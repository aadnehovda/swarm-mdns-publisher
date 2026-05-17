package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/brutella/dnssd"
)

func main() {
	serviceType := "_https._tcp"
	if len(os.Args) > 1 {
		serviceType = os.Args[1]
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := dnssd.LookupType(ctx, serviceType, func(entry dnssd.BrowseEntry) {
		service, err := dnssd.LookupInstance(ctx, entry.ServiceInstanceName())
		if err != nil {
			fmt.Printf("name=%s resolve_error=%v\n", entry.ServiceInstanceName(), err)
			return
		}
		fmt.Printf("name=%s host=%s addr=%v port=%d text=%v\n", service.ServiceInstanceName(), service.Hostname(), service.IPs, service.Port, service.Text)
	}, func(entry dnssd.BrowseEntry) {})
	if err != nil && ctx.Err() == nil {
		fmt.Fprintf(os.Stderr, "lookup failed: %v\n", err)
		os.Exit(1)
	}
}
