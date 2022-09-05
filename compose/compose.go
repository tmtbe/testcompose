package compose

import (
	"github.com/google/uuid"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
	"podcompose/docker"
)

type Compose struct {
	podCompose *PodCompose
	config     *ComposeConfig
	provider   *docker.DockerProvider
	volume     *Volume
}

func NewCompose(configBytes []byte) (*Compose, error) {
	sessionID, _ := uuid.NewUUID()
	var config ComposeConfig
	err := yaml.Unmarshal(configBytes, &config)
	if err != nil {
		return nil, err
	}
	if config.SessionId == "" {
		config.SessionId = sessionID.String()
	}
	err = config.check()
	if err != nil {
		return nil, err
	}
	provider, err := docker.NewDockerProvider()
	if err != nil {
		return nil, err
	}
	compose, err := NewPodCompose(sessionID.String(), config.Pods, config.getNetworkName(), provider)
	if err != nil {
		return nil, err
	}
	return &Compose{
		podCompose: compose,
		config:     &config,
		provider:   provider,
		volume:     NewVolumes(config.Volumes, provider),
	}, nil
}

// PrepareNetworkAndVolumes network and volumes should be init before agent start
func (c *Compose) PrepareNetworkAndVolumes(ctx context.Context) error {
	_, err := c.provider.CreateNetwork(ctx, docker.NetworkRequest{
		Driver:         docker.Bridge,
		CheckDuplicate: true,
		Name:           c.config.getNetworkName(),
	}, c.config.SessionId)
	if err != nil {
		return err
	}
	err = c.volume.createVolumes(ctx, c.config.SessionId)
	if err != nil {
		return err
	}
	return nil
}

func (c *Compose) Clean(ctx context.Context) {
	c.podCompose.clean(ctx)
	c.volume.clean(ctx)
}

func (c *Compose) StartPods(ctx context.Context) error {
	return c.podCompose.start(ctx)
}
