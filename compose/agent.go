package compose

import (
	"context"
	"encoding/json"
	"github.com/pkg/errors"
	"io/ioutil"
	"path/filepath"
	"podcompose/common"
	"podcompose/docker"
	"podcompose/docker/wait"
	"strings"
)

const AgentAutoRemove = true

type Agent struct {
	composeProvider ComposeProvider
}
type ComposeProvider interface {
	GetContextPathForMount() string
	GetDockerProvider() *docker.DockerProvider
	GetSessionId() string
	GetConfig() *ComposeConfig
}

func NewAgent(composeProvider ComposeProvider) *Agent {
	return &Agent{
		composeProvider: composeProvider,
	}
}

func (a *Agent) GetSessionId() string {
	return a.composeProvider.GetSessionId()
}
func (a *Agent) StartAgentForServer(ctx context.Context) (docker.Container, error) {
	agentMounts := make([]docker.ContainerMount, 0)
	agentMounts = append(agentMounts, docker.BindMount("/var/run/docker.sock", "/var/run/docker.sock"))
	agentMounts = append(agentMounts, docker.BindMount(a.composeProvider.GetContextPathForMount(), common.AgentContextPath))
	return a.composeProvider.GetDockerProvider().RunContainer(ctx, docker.ContainerRequest{
		Image:        common.AgentImage,
		Name:         ContainerNamePrefix + "agent_" + a.composeProvider.GetSessionId(),
		ExposedPorts: []string{common.AgentPort},
		Mounts:       agentMounts,
		WaitingFor: wait.ForHTTP(common.AgentHealthEndPoint).
			WithPort(common.AgentPort + "/tcp").
			WithMethod("GET"),
		Env: map[string]string{
			common.AgentSessionID:  a.composeProvider.GetSessionId(),
			common.HostContextPath: a.composeProvider.GetContextPathForMount(),
		},
		Networks: []string{a.composeProvider.GetDockerProvider().GetDefaultNetwork(), a.composeProvider.GetConfig().GetNetworkName()},
		NetworkAliases: map[string][]string{
			a.composeProvider.GetConfig().GetNetworkName(): {"agent"},
		},
		Cmd: []string{"start"},
		Labels: map[string]string{
			docker.AgentType: docker.AgentTypeServer,
		},
		AutoRemove: AgentAutoRemove,
	}, a.composeProvider.GetSessionId())
}

func (a *Agent) StartAgentForSetVolume(ctx context.Context, selectData map[string]string) error {
	agentMounts := make([]docker.ContainerMount, 0)
	agentMounts = append(agentMounts, docker.BindMount("/var/run/docker.sock", "/var/run/docker.sock"))
	agentMounts = append(agentMounts, docker.BindMount(a.composeProvider.GetContextPathForMount(), common.AgentContextPath))

	cmd := make([]string, 0)
	cmd = append(cmd, "prepareVolumeData")
	for volumeName, selectDataName := range selectData {
		cmd = append(cmd, "-s")
		cmd = append(cmd, volumeName+"="+selectDataName)
	}
	for _, volume := range a.composeProvider.GetConfig().Volumes {
		if _, ok := selectData[volume.Name]; ok {
			volumeName := volume.Name + "_" + a.composeProvider.GetSessionId()
			agentMounts = append(agentMounts, docker.VolumeMount(volumeName, docker.ContainerMountTarget(common.AgentVolumePath+volume.Name)))
		}
	}
	return a.runAndGetAgentError(ctx, docker.ContainerRequest{
		Image: common.AgentImage,
		Name:  ContainerNamePrefix + "agent_volume_" + a.composeProvider.GetSessionId(),
		Env: map[string]string{
			common.AgentSessionID:  a.composeProvider.GetSessionId(),
			common.HostContextPath: a.composeProvider.GetContextPathForMount(),
		},
		Mounts: agentMounts,
		Cmd:    cmd,
		Labels: map[string]string{
			docker.AgentType: docker.AgentTypeVolume,
		},
	}, AgentAutoRemove)
}

func (a *Agent) StartAgentForClean(ctx context.Context) error {
	return a.runAndGetAgentError(ctx, docker.ContainerRequest{
		Image:  common.AgentImage,
		Mounts: docker.Mounts(docker.BindMount("/var/run/docker.sock", "/var/run/docker.sock")),
		Name:   ContainerNamePrefix + "agent_clean_" + a.composeProvider.GetSessionId(),
		Env: map[string]string{
			common.AgentSessionID: a.composeProvider.GetSessionId(),
		},
		Cmd: []string{"clean"},
		Labels: map[string]string{
			docker.AgentType: docker.AgentTypeCleaner,
		},
		AutoRemove: AgentAutoRemove, //clean must set auto remove,agent cannot remove clean container
	}, AgentAutoRemove)
}

func (a *Agent) StartAgentForSwitchData(ctx context.Context, selectData map[string]string) error {
	agentMounts := make([]docker.ContainerMount, 0)
	agentMounts = append(agentMounts, docker.BindMount("/var/run/docker.sock", "/var/run/docker.sock"))
	agentMounts = append(agentMounts, docker.BindMount(a.composeProvider.GetContextPathForMount(), common.AgentContextPath))

	cmd := make([]string, 0)
	cmd = append(cmd, "switch")
	for volumeName, selectDataName := range selectData {
		cmd = append(cmd, "-s")
		cmd = append(cmd, volumeName+"="+selectDataName)
	}
	for _, volume := range a.composeProvider.GetConfig().Volumes {
		if _, ok := selectData[volume.Name]; ok {
			volumeName := volume.Name + "_" + a.composeProvider.GetSessionId()
			agentMounts = append(agentMounts, docker.VolumeMount(volumeName, docker.ContainerMountTarget(common.AgentVolumePath+volume.Name)))
		}
	}
	return a.runAndGetAgentError(ctx, docker.ContainerRequest{
		Image: common.AgentImage,
		Name:  ContainerNamePrefix + "agent_switch_" + a.composeProvider.GetSessionId(),
		Env: map[string]string{
			common.AgentSessionID:  a.composeProvider.GetSessionId(),
			common.HostContextPath: a.composeProvider.GetContextPathForMount(),
		},
		Mounts: agentMounts,
		Cmd:    cmd,
		Labels: map[string]string{
			docker.AgentType: docker.AgentTypeSwitchData,
		},
	}, AgentAutoRemove)
}

func (a *Agent) StartAgentForRestart(ctx context.Context, selectData []string) error {
	agentMounts := make([]docker.ContainerMount, 0)
	agentMounts = append(agentMounts, docker.BindMount("/var/run/docker.sock", "/var/run/docker.sock"))
	agentMounts = append(agentMounts, docker.BindMount(a.composeProvider.GetContextPathForMount(), common.AgentContextPath))
	cmd := make([]string, 0)
	cmd = append(cmd, "restart")
	for _, podName := range selectData {
		cmd = append(cmd, "-s")
		cmd = append(cmd, podName)
	}
	return a.runAndGetAgentError(ctx, docker.ContainerRequest{
		Image: common.AgentImage,
		Name:  ContainerNamePrefix + "agent_restart_" + a.composeProvider.GetSessionId(),
		Env: map[string]string{
			common.AgentSessionID:  a.composeProvider.GetSessionId(),
			common.HostContextPath: a.composeProvider.GetContextPathForMount(),
		},
		Mounts: agentMounts,
		Cmd:    cmd,
	}, AgentAutoRemove)
}

func (a *Agent) startAgentForIngressSetVolume(ctx context.Context, volumeName string, servicePortInfo map[string]string) error {
	agentMounts := make([]docker.ContainerMount, 0)
	agentMounts = append(agentMounts, docker.BindMount("/var/run/docker.sock", "/var/run/docker.sock"))
	agentMounts = append(agentMounts, docker.VolumeMount(volumeName, docker.ContainerMountTarget(filepath.Join(common.AgentVolumePath, common.IngressVolumeName))))
	cmd := make([]string, 0)
	cmd = append(cmd, "prepareIngressVolume")
	for serviceName, portMapping := range servicePortInfo {
		cmd = append(cmd, "-p")
		cmd = append(cmd, serviceName+"="+portMapping)
	}
	return a.runAndGetAgentError(ctx, docker.ContainerRequest{
		Image: common.AgentImage,
		Name:  ContainerNamePrefix + "agent_ingress_volume" + a.composeProvider.GetSessionId(),
		Env: map[string]string{
			common.AgentSessionID:  a.composeProvider.GetSessionId(),
			common.HostContextPath: a.composeProvider.GetContextPathForMount(),
		},
		Mounts: agentMounts,
		Cmd:    cmd,
		Labels: map[string]string{
			docker.AgentType: docker.AgentTypeIngressVolume,
		},
	}, AgentAutoRemove)
}

func (a *Agent) StartAgentForIngress(ctx context.Context, servicePortInfo map[string]string) (docker.Container, error) {
	volumeName := common.IngressVolumeName + "_" + a.GetSessionId()
	containerName := ContainerNamePrefix + "agent_ingress_" + a.composeProvider.GetSessionId()
	// first remove ingress container
	ingressContainer, _ := a.composeProvider.GetDockerProvider().FindContainerByName(ctx, containerName)
	if ingressContainer != nil {
		_ = a.composeProvider.GetDockerProvider().RemoveContainer(ctx, ingressContainer.ID)
	}
	// then remove ingress volume
	_ = a.composeProvider.GetDockerProvider().RemoveVolume(ctx, volumeName, true)
	// create ingress volume
	_, err := a.composeProvider.GetDockerProvider().CreateVolume(ctx, volumeName, a.GetSessionId())
	if err != nil {
		return nil, err
	}
	// prepare volume
	err = a.startAgentForIngressSetVolume(ctx, volumeName, servicePortInfo)
	if err != nil {
		return nil, err
	}
	// start ingress container
	exposePorts := make([]string, 0)
	for _, portInfo := range servicePortInfo {
		ports := strings.SplitN(portInfo, ":", 2)
		exposePorts = append(exposePorts, ports[1]+":"+ports[1])
	}
	return a.composeProvider.GetDockerProvider().RunContainer(ctx, docker.ContainerRequest{
		Image:        common.IngressImage,
		Name:         containerName,
		Mounts:       docker.Mounts(docker.VolumeMount(volumeName, "/etc/envoy")),
		ExposedPorts: exposePorts,
		Labels: map[string]string{
			docker.AgentType: docker.AgentTypeIngress,
		},
		Networks: []string{a.composeProvider.GetConfig().GetNetworkName()},
	}, a.GetSessionId())
}

// must use waitingFor exit
func (a *Agent) runAndGetAgentError(ctx context.Context, containerRequest docker.ContainerRequest, remove bool) error {
	containerRequest.WaitingFor = wait.ForExit()
	container, err := a.composeProvider.GetDockerProvider().CreateContainerAutoLabel(ctx, containerRequest, a.composeProvider.GetSessionId())
	if err != nil {
		return err
	}
	if err := container.Start(ctx); err != nil {
		return err
	}
	// if auto remove, we can not get logs, so just return
	if containerRequest.AutoRemove {
		return nil
	}
	if remove {
		defer a.composeProvider.GetDockerProvider().RemoveContainer(ctx, container.GetContainerID())
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
