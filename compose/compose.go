package compose

import (
	"encoding/json"
	"github.com/pkg/errors"
	"github.com/sony/sonyflake"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"path/filepath"
	"podcompose/common"
	"podcompose/docker"
	"podcompose/docker/wait"
	"strconv"
	"strings"
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

func (c *Compose) StartAgentForServer(ctx context.Context) (docker.Container, error) {
	agentMounts := make([]docker.ContainerMount, 0)
	agentMounts = append(agentMounts, docker.BindMount("/var/run/docker.sock", "/var/run/docker.sock"))
	agentMounts = append(agentMounts, docker.BindMount(c.getContextPathForMount(), common.AgentContextPath))
	return c.GetDockerProvider().RunContainer(ctx, docker.ContainerRequest{
		Image:        common.AgentImage,
		Name:         ContainerNamePrefix + "agent_" + c.GetConfig().SessionId,
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
	return c.runAndGetAgentError(ctx, docker.ContainerRequest{
		Image: common.AgentImage,
		Name:  ContainerNamePrefix + "agent_volume_" + c.GetConfig().SessionId,
		Env: map[string]string{
			common.AgentSessionID:  c.GetConfig().SessionId,
			common.HostContextPath: c.getContextPathForMount(),
		},
		Mounts: agentMounts,
		Cmd:    cmd,
	}, AgentAutoRemove)
}

func (c *Compose) StartAgentForClean(ctx context.Context) error {
	return c.runAndGetAgentError(ctx, docker.ContainerRequest{
		Image:  common.AgentImage,
		Mounts: docker.Mounts(docker.BindMount("/var/run/docker.sock", "/var/run/docker.sock")),
		Name:   ContainerNamePrefix + "agent_clean_" + c.GetConfig().SessionId,
		Env: map[string]string{
			common.AgentSessionID: c.GetConfig().SessionId,
		},
		Cmd: []string{"clean"},
		Labels: map[string]string{
			docker.IsCleaner: "true",
		},
		AutoRemove: AgentAutoRemove, //clean must set auto remove,agent cannot remove clean container
	}, AgentAutoRemove)
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
	return c.runAndGetAgentError(ctx, docker.ContainerRequest{
		Image: common.AgentImage,
		Name:  ContainerNamePrefix + "agent_switch_" + c.GetConfig().SessionId,
		Env: map[string]string{
			common.AgentSessionID:  c.GetConfig().SessionId,
			common.HostContextPath: c.getContextPathForMount(),
		},
		Mounts: agentMounts,
		Cmd:    cmd,
	}, AgentAutoRemove)
}

func (c *Compose) StartAgentForRestart(ctx context.Context, selectData []string) error {
	agentMounts := make([]docker.ContainerMount, 0)
	agentMounts = append(agentMounts, docker.BindMount("/var/run/docker.sock", "/var/run/docker.sock"))
	agentMounts = append(agentMounts, docker.BindMount(c.getContextPathForMount(), common.AgentContextPath))
	cmd := make([]string, 0)
	cmd = append(cmd, "restart")
	for _, podName := range selectData {
		cmd = append(cmd, "-s")
		cmd = append(cmd, podName)
	}
	return c.runAndGetAgentError(ctx, docker.ContainerRequest{
		Image: common.AgentImage,
		Name:  ContainerNamePrefix + "agent_restart_" + c.GetConfig().SessionId,
		Env: map[string]string{
			common.AgentSessionID:  c.GetConfig().SessionId,
			common.HostContextPath: c.getContextPathForMount(),
		},
		Mounts: agentMounts,
		Cmd:    cmd,
	}, AgentAutoRemove)
}

// must use waitingFor exit
func (c *Compose) runAndGetAgentError(ctx context.Context, containerRequest docker.ContainerRequest, remove bool) error {
	containerRequest.WaitingFor = wait.ForExit()
	container, err := c.GetDockerProvider().CreateContainerAutoLabel(ctx, containerRequest, c.config.SessionId)
	if err != nil {
		return err
	}
	if err := container.Start(ctx); err != nil {
		return err
	}
	if remove {
		defer c.dockerProvider.RemoveContainer(ctx, container.GetContainerID())
	}
	state, err := container.State(ctx)
	if err != nil {
		return err
	}
	if state.ExitCode != 0 {
		logs, err := container.Logs(ctx)
		if err != nil {
			return err
		}
		all, err := ioutil.ReadAll(logs)
		if err != nil {
			return err
		}
		log := string(all)
		lines := strings.Split(log, "\n")
		for _, line := range lines {
			split := strings.SplitN(line, "{", 2)
			jsonLog := "{" + split[1]
			var logStruct struct {
				Level string `json:"level"`
				Msg   string `json:"msg"`
			}
			err := json.Unmarshal([]byte(jsonLog), &logStruct)
			if err == nil {
				if logStruct.Level == "error" {
					return errors.New(logStruct.Msg)
				}
			}
		}
		return errors.New(log)
	}
	return nil
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

func (c *Compose) getContextPathForMount() string {
	if c.hostContextPath != "" {
		return c.hostContextPath
	} else {
		return c.contextPath
	}
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
