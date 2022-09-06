package compose

import (
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
	"path/filepath"
	"podcompose/common"
	"podcompose/docker"
)

type Compose struct {
	podCompose     *PodCompose
	config         *ComposeConfig
	dockerProvider *docker.DockerProvider
	volume         *Volume
}

func NewCompose(configBytes []byte, sessionId string, contextPath string) (*Compose, error) {
	contextPath, err := filepath.Abs(contextPath)
	if err != nil {
		return nil, err
	}
	var config ComposeConfig
	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		return nil, err
	}
	config.SessionId = sessionId
	if config.SessionId == "" {
		sessionUUID, _ := uuid.NewUUID()
		config.SessionId = sessionUUID.String()
	}
	err = config.check(contextPath)
	if err != nil {
		return nil, err
	}
	provider, err := docker.NewDockerProvider()
	if err != nil {
		return nil, err
	}
	compose, err := NewPodCompose(sessionId, config.Pods, config.GetNetworkName(), provider)
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
	if c.config.Network == "" {
		_, err := c.dockerProvider.CreateNetwork(ctx, docker.NetworkRequest{
			Driver:         docker.Bridge,
			CheckDuplicate: true,
			Name:           c.config.GetNetworkName(),
		}, c.config.SessionId)
		if err != nil {
			return err
		}
	} else {
		_, err := c.dockerProvider.GetNetwork(ctx, docker.NetworkRequest{
			Name: c.config.Network,
		})
		if err != nil {
			return errors.Errorf("network: %s is not exist", c.config.Network)
		}
	}
	err := c.volume.createVolumes(ctx, c.config.SessionId)
	if err != nil {
		return err
	}
	return nil
}

func (c *Compose) Clean(ctx context.Context) error {
	cc, err := c.GetDockerProvider().CreateContainer(ctx, docker.ContainerRequest{
		Image:  common.AgentImage,
		Mounts: docker.Mounts(docker.BindMount("/var/run/docker.sock", "/var/run/docker.sock")),
		Name:   "agent_clean_" + c.GetConfig().SessionId,
		Env: map[string]string{
			common.AgentSessionID: c.GetConfig().SessionId,
		},
		Cmd:        []string{"clean"},
		AutoRemove: true,
	}, c.GetConfig().SessionId, false)
	if err != nil {
		return err
	}
	return cc.Start(ctx)
}

func (c *Compose) StartPods(ctx context.Context) error {
	return c.podCompose.start(ctx)
}
