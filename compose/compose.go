package compose

import (
	"github.com/google/uuid"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
	"podcompose/docker"
)

type Compose struct {
	podCompose     *PodCompose
	config         *ComposeConfig
	dockerProvider *docker.DockerProvider
	volume         *Volume
}

func NewCompose(configBytes []byte, sessionId string, contextPath string) (*Compose, error) {
	sessionID, _ := uuid.NewUUID()
	var config ComposeConfig
	err := yaml.Unmarshal(configBytes, &config)
	if err != nil {
		return nil, err
	}
	config.SessionId = sessionId
	if config.SessionId == "" {
		config.SessionId = sessionID.String()
	}
	err = config.check(contextPath)
	if err != nil {
		return nil, err
	}
	provider, err := docker.NewDockerProvider()
	if err != nil {
		return nil, err
	}
	compose, err := NewPodCompose(sessionID.String(), config.Pods, config.GetNetworkName(), provider)
	if err != nil {
		return nil, err
	}
	return &Compose{
		podCompose:     compose,
		config:         &config,
		dockerProvider: provider,
		volume:         NewVolumes(config.Volumes, provider),
	}, nil
}

func (c *Compose) GetConfig() *ComposeConfig {
	return c.config
}

func (c *Compose) GetDockerProvider() *docker.DockerProvider {
	return c.dockerProvider
}

// PrepareNetworkAndVolumes network and volumes should be init before agent start
func (c *Compose) PrepareNetworkAndVolumes(ctx context.Context) error {
	_, err := c.dockerProvider.CreateNetwork(ctx, docker.NetworkRequest{
		Driver:         docker.Bridge,
		CheckDuplicate: true,
		Name:           c.config.GetNetworkName(),
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
