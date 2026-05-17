package publisher

import (
	"context"

	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
)

type DockerClient struct {
	client *client.Client
}

func NewDockerClient(host string) (*DockerClient, error) {
	opts := []client.Opt{
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	}
	if host != "" {
		opts = append(opts, client.WithHost(host))
	}

	dockerClient, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, err
	}
	return &DockerClient{client: dockerClient}, nil
}

func (c *DockerClient) ListServices(ctx context.Context) ([]swarm.Service, error) {
	return c.client.ServiceList(ctx, swarm.ServiceListOptions{})
}

func (c *DockerClient) WatchServiceEvents(ctx context.Context) (<-chan events.Message, <-chan error) {
	return c.client.Events(ctx, events.ListOptions{
		Filters: filters.NewArgs(filters.Arg("type", "service")),
	})
}

func (c *DockerClient) Close() error {
	if c.client == nil {
		return nil
	}
	return c.client.Close()
}
