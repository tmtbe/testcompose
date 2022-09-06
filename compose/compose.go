package compose

import (
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
	"path/filepath"
	"podcompose/common"
	"podcompose/docker"
	"podcompose/docker/wait"
)

const AgentAutoRemove = true

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
		contextPath:    contextPath,
	}, nil
}

func (c *Compose) GetConfig() *ComposeConfig {
	return c.config
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

func (c *Compose) StartAgentForServer(ctx context.Context) (docker.Container, error) {
	agentMounts := make([]docker.ContainerMount, 0)
	agentMounts = append(agentMounts, docker.BindMount("/var/run/docker.sock", "/var/run/docker.sock"))
	agentMounts = append(agentMounts, docker.BindMount(c.getContextPathForMount(), common.AgentContextPath))
	return c.GetDockerProvider().RunContainer(ctx, docker.ContainerRequest{
		Image:        common.AgentImage,
		Name:         "agent_" + c.GetConfig().SessionId,
		ExposedPorts: []string{common.AgentPort},
		Mounts:       agentMounts,
		WaitingFor: wait.ForHTTP(common.AgentHealthEndPoint).
			WithPort(common.AgentPort + "/tcp").
			WithMethod("GET"),
		Env: map[string]string{
			common.AgentSessionID:  c.GetConfig().SessionId,
			common.HostContextPath: c.getContextPathForMount(),
		},
		Networks: []string{c.GetDockerProvider().GetDefaultNetwork(), c.GetConfig().GetNetworkName()},
		NetworkAliases: map[string][]string{
			c.GetConfig().GetNetworkName(): {"agent"},
		},
		Cmd:        []string{"start"},
		AutoRemove: AgentAutoRemove,
	}, c.GetConfig().SessionId)
}

func (c *Compose) SetHostContextPath(path string) {
	c.hostContextPath = path
}

func (c *Compose) getContextPathForMount() string {
	if c.hostContextPath != "" {
		return c.hostContextPath
	} else {
		return c.contextPath
	}
}
func (c *Compose) StartAgentForSetVolume(ctx context.Context, selectData map[string]string) error {
	agentMounts := make([]docker.ContainerMount, 0)
	agentMounts = append(agentMounts, docker.BindMount("/var/run/docker.sock", "/var/run/docker.sock"))
	agentMounts = append(agentMounts, docker.BindMount(c.getContextPathForMount(), common.AgentContextPath))

	cmd := make([]string, 0)
	cmd = append(cmd, "prepareVolumeData")
	for volumeName, selectDataName := range selectData {
		cmd = append(cmd, "-s")
		cmd = append(cmd, volumeName+"="+selectDataName)
	}
	for _, volume := range c.GetConfig().Volumes {
		if _, ok := selectData[volume.Name]; ok {
			volumeName := volume.Name + "_" + c.GetConfig().SessionId
			agentMounts = append(agentMounts, docker.VolumeMount(volumeName, docker.ContainerMountTarget(common.AgentVolumePath+volume.Name)))
		}
	}
	cc, err := c.GetDockerProvider().CreateContainer(ctx, docker.ContainerRequest{
		Image: common.AgentImage,
		Name:  "agent_volume_" + c.GetConfig().SessionId,
		Env: map[string]string{
			common.AgentSessionID:  c.GetConfig().SessionId,
			common.HostContextPath: c.getContextPathForMount(),
		},
		Mounts:     agentMounts,
		WaitingFor: wait.ForExit(),
		Cmd:        cmd,
		AutoRemove: AgentAutoRemove,
	}, c.GetConfig().SessionId, false)
	if err != nil {
		return err
	}
	return cc.Start(ctx)
}

func (c *Compose) StartAgentForClean(ctx context.Context) error {
	cc, err := c.GetDockerProvider().CreateContainer(ctx, docker.ContainerRequest{
		Image:  common.AgentImage,
		Mounts: docker.Mounts(docker.BindMount("/var/run/docker.sock", "/var/run/docker.sock")),
		Name:   "agent_clean_" + c.GetConfig().SessionId,
		Env: map[string]string{
			common.AgentSessionID: c.GetConfig().SessionId,
		},
		Cmd:        []string{"clean"},
		AutoRemove: AgentAutoRemove,
	}, c.GetConfig().SessionId, false)
	if err != nil {
		return err
	}
	return cc.Start(ctx)
}

func (c *Compose) StartAgentForSwitchData(ctx context.Context, selectData map[string]string) error {
	agentMounts := make([]docker.ContainerMount, 0)
	agentMounts = append(agentMounts, docker.BindMount("/var/run/docker.sock", "/var/run/docker.sock"))
	agentMounts = append(agentMounts, docker.BindMount(c.getContextPathForMount(), common.AgentContextPath))

	cmd := make([]string, 0)
	cmd = append(cmd, "switch")
	for volumeName, selectDataName := range selectData {
		cmd = append(cmd, "-s")
		cmd = append(cmd, volumeName+"="+selectDataName)
	}
	for _, volume := range c.GetConfig().Volumes {
		if _, ok := selectData[volume.Name]; ok {
			volumeName := volume.Name + "_" + c.GetConfig().SessionId
			agentMounts = append(agentMounts, docker.VolumeMount(volumeName, docker.ContainerMountTarget(common.AgentVolumePath+volume.Name)))
		}
	}
	cc, err := c.GetDockerProvider().CreateContainer(ctx, docker.ContainerRequest{
		Image: common.AgentImage,
		Name:  "agent_switch_" + c.GetConfig().SessionId,
		Env: map[string]string{
			common.AgentSessionID:  c.GetConfig().SessionId,
			common.HostContextPath: c.getContextPathForMount(),
		},
		Mounts:     agentMounts,
		WaitingFor: wait.ForExit(),
		Cmd:        cmd,
		AutoRemove: AgentAutoRemove,
	}, c.GetConfig().SessionId, false)
	if err != nil {
		return err
	}
	return cc.Start(ctx)
}

func (c *Compose) StartPods(ctx context.Context) error {
	return c.podCompose.start(ctx)
}

func (c *Compose) CreateVolumes(ctx context.Context) error {
	return c.volume.createVolumes(ctx, c.GetConfig().SessionId)
}

func (c *Compose) ReCreateVolumes(ctx context.Context, names []string) error {
	return c.volume.reCreateVolumes(ctx, names, c.GetConfig().SessionId)
}

func (c *Compose) FindPodsWhoUsedVolumes(volumeNames []string) []*PodConfig {
	return c.podCompose.findPodsWhoUsedVolumes(volumeNames)
}

func (c *Compose) RestartPods(ctx context.Context, pods []*PodConfig, beforeStart func() error) error {
	podNames := make([]string, len(pods))
	for k, v := range pods {
		podNames[k] = v.Name
	}
	return c.podCompose.RestartPods(ctx, podNames, beforeStart)
}
