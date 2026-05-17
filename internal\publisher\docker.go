package publisher

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const defaultDockerSocket = "/var/run/docker.sock"

type DockerClient struct {
	httpClient *http.Client
	baseURL    string
}

type DockerService struct {
	ID       string      `json:"ID"`
	Spec     ServiceSpec `json:"Spec"`
	Endpoint Endpoint    `json:"Endpoint"`
}

type ServiceSpec struct {
	Name   string            `json:"Name"`
	Labels map[string]string `json:"Labels"`
}

type Endpoint struct {
	Ports []PortConfig `json:"Ports"`
}

type PortConfig struct {
	Protocol      string `json:"Protocol"`
	TargetPort    uint32 `json:"TargetPort"`
	PublishedPort uint32 `json:"PublishedPort"`
	PublishMode   string `json:"PublishMode"`
}

type DockerEvent struct {
	Type   string     `json:"Type"`
	Action string     `json:"Action"`
	Actor  EventActor `json:"Actor"`
}

type EventActor struct {
	ID string `json:"ID"`
}

func NewDockerClient(host string) (*DockerClient, error) {
	if host == "" {
		host = os.Getenv("DOCKER_HOST")
	}
	if host == "" {
		host = "unix://" + defaultDockerSocket
	}
	if !strings.HasPrefix(host, "unix://") {
		return nil, fmt.Errorf("only unix Docker sockets are supported, got %q", host)
	}

	socketPath := strings.TrimPrefix(host, "unix://")
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}

	return &DockerClient{
		httpClient: &http.Client{Transport: transport},
		baseURL:    "http://docker/v1.41",
	}, nil
}

func (c *DockerClient) ListServices(ctx context.Context) ([]DockerService, error) {
	var services []DockerService
	if err := c.getJSON(ctx, "/services", &services); err != nil {
		return nil, err
	}
	return services, nil
}

func (c *DockerClient) WatchServiceEvents(ctx context.Context) (<-chan DockerEvent, <-chan error) {
	events := make(chan DockerEvent)
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errs)

		query := url.Values{}
		query.Set("filters", `{"type":["service"]}`)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/events?"+query.Encode(), nil)
		if err != nil {
			errs <- err
			return
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			errs <- err
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			errs <- fmt.Errorf("Docker events returned %s", resp.Status)
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			var event DockerEvent
			if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
				errs <- err
				return
			}

			select {
			case events <- event:
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil && ctx.Err() == nil {
			errs <- err
		}
	}()

	return events, errs
}

func (c *DockerClient) getJSON(ctx context.Context, path string, target any) error {
	reqCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("Docker API %s returned %s", path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}
