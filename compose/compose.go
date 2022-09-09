package compose

import (
	"github.com/pkg/errors"
	"github.com/sony/sonyflake"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
	"path/filepath"
	"podcompose/docker"
	"strconv"
)

type Compose struct {
	podCompose      *PodCompose
	config          *ComposeConfig
	dockerProvider  *docker.DockerProvider
	volume          *Volume
	contextPath     string
	hostContextPath string
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
		config.SessionId = genSessionId()
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
		contextPath:    contextPath,
	}, nil
}

func genSessionId() string {
	var st sonyflake.Settings
	id, _ := sonyflake.NewSonyflake(st).NextID()
	return strconv.FormatInt(int64(id), 16)
}

func (c *Compose) StartPods(ctx context.Context) error {
	return c.podCompose.start(ctx)
}

func (c *Compose) CreateVolumes(ctx context.Context) error {
	return c.volume.createVolumes(ctx, c.GetConfig().SessionId)
}

func (c *Compose) RecreateVolumes(ctx context.Context, names []string) error {
	return c.volume.recreateVolumes(ctx, names, c.GetConfig().SessionId)
}

func (c *Compose) FindPodsWhoUsedVolumes(volumeNames []string) []*PodConfig {
	return c.podCompose.findPodsWhoUsedVolumes(volumeNames)
}

func (c *Compose) RestartPods(ctx context.Context, podNames []string, beforeStart func() error) error {
	for _, podName := range podNames {
		if _, ok := c.podCompose.pods[podName]; !ok {
			return errors.Errorf("pod name:%s is not exist", podName)
		}
	}
	return c.podCompose.RestartPods(ctx, podNames, beforeStart)
}

func (c *Compose) SetHostContextPath(path string) {
	c.hostContextPath = path
}

func (c *Compose) GetContextPathForMount() string {
	if c.hostContextPath != "" {
		return c.hostContextPath
	} else {
		return c.contextPath
	}
}

func (c *Compose) GetConfig() *ComposeConfig {
	return c.config
}

func (c *Compose) GetSessionId() string {
	return c.config.SessionId
}

func (c *Compose) GetDockerProvider() *docker.DockerProvider {
	return c.dockerProvider
}

// PrepareNetwork network and volumes should be init before agent start
func (c *Compose) PrepareNetwork(ctx context.Context) error {
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
	return nil
}
