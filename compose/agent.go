package compose

import (
	"context"
	"github.com/pkg/errors"
	"os"
	"path/filepath"
	"podcompose/common"
	"podcompose/config"
	"podcompose/docker"
	"podcompose/docker/wait"
	"strconv"
	"strings"
)

type Agent struct {
	composeProvider ComposeProvider
}
type Info struct {
	SessionId   string
	IsReady     bool
	VolumeInfos []VolumeInfo
	PodInfos    []PodInfo
}
type PodInfo struct {
	Name           string
	ContainerInfos []ContainerInfo
}
type ContainerInfo struct {
	Name        string
	ContainerId string
	State       string
	Image       string
	Created     int64
}
type VolumeInfo struct {
	Name     string
	VolumeId string
}

type ComposeProvider interface {
	GetContextPathForMount() string
	GetDockerProvider() *docker.DockerProvider
	GetSessionId() string
	GetConfig() *ComposeConfig
	IsReady() bool
}

func NewAgent(composeProvider ComposeProvider) *Agent {
	return &Agent{
		composeProvider: composeProvider,
	}
}

func (a *Agent) GetSessionId() string {
	return a.composeProvider.GetSessionId()
}
func (a *Agent) GetInfo() Info {
	var volumeInfos []VolumeInfo
	if len(a.composeProvider.GetConfig().VolumeGroups) > 0 {
		volumeInfos := make([]VolumeInfo, len(a.composeProvider.GetConfig().VolumeGroups[0].Volumes))
		for i, v := range a.composeProvider.GetConfig().VolumeGroups[0].Volumes {
			volumeInfos[i] = VolumeInfo{
				Name:     v.Name,
				VolumeId: v.Name + "_" + genSessionId(),
			}
		}
	}
	ctx := context.Background()
	containers, _ := a.composeProvider.GetDockerProvider().FindAllContainersWithSessionId(ctx, a.composeProvider.GetSessionId())
	podInfos := make([]PodInfo, len(a.composeProvider.GetConfig().Pods))
	for i, p := range a.composeProvider.GetConfig().Pods {
		containerInfos := make([]ContainerInfo, 0)
		for _, c := range containers {
			if c.Labels[common.LabelPodName] == p.Name {
				containerInfos = append(containerInfos, ContainerInfo{
					Name:        c.Labels[common.LabelContainerName],
					ContainerId: c.ID,
					State:       c.State,
					Image:       c.Image,
					Created:     c.Created,
				})
			}
		}
		podInfos[i] = PodInfo{
			Name:           p.Name,
			ContainerInfos: containerInfos,
		}
	}
	return Info{
		SessionId:   genSessionId(),
		VolumeInfos: volumeInfos,
		PodInfos:    podInfos,
		IsReady:     a.composeProvider.IsReady(),
	}
}
func (a *Agent) StartAgentForServer(ctx context.Context, autoStart bool, bootInDocker bool) (docker.Container, error) {
	agentMounts := make([]docker.ContainerMount, 0)
	agentMounts = append(agentMounts, docker.BindMount("/var/run/docker.sock", "/var/run/docker.sock"))
	agentMounts = append(agentMounts, docker.BindMount(a.composeProvider.GetContextPathForMount(), common.AgentContextPath))
	agentMounts = append(agentMounts, docker.VolumeMount(common.SystemLogVolumeName+"_"+a.GetSessionId(), common.AgentLogPath))
	containerName := common.ContainerNamePrefix + "agent_" + a.composeProvider.GetSessionId()
	var waitStrategy wait.Strategy
	if bootInDocker {
		waitStrategy = wait.ForNetworkHTTP(common.EndPointAgentHealth, "agent").
			WithPort(common.ServerAgentPort + "/tcp").
			WithMethod("GET")
	} else {
		waitStrategy = wait.ForHTTP(common.EndPointAgentHealth).
			WithPort(common.ServerAgentPort + "/tcp").
			WithMethod("GET")
	}
	return a.composeProvider.GetDockerProvider().RunContainer(ctx, docker.ContainerRequest{
		Image:        config.ComposeConfig.Image.Agent,
		Name:         containerName,
		ExposedPorts: []string{common.ServerAgentPort, common.ServerAgentEventBusPort},
		Mounts:       agentMounts,
		WaitingFor:   waitStrategy,
		Env: map[string]string{
			common.LabelSessionID:     a.composeProvider.GetSessionId(),
			common.EnvHostContextPath: a.composeProvider.GetContextPathForMount(),
			common.TpcDebug:           os.Getenv(common.TpcDebug),
			common.TpcName:            containerName,
		},
		Networks: []string{a.composeProvider.GetDockerProvider().GetDefaultNetwork(), a.composeProvider.GetConfig().Network},
		NetworkAliases: map[string][]string{
			a.composeProvider.GetConfig().Network: {"agent"},
		},
		Cmd: []string{"start", "--autoStart=" + strconv.FormatBool(autoStart), "--fromConfigJson", config.GetConfigJson()},
		Labels: map[string]string{
			docker.AgentType: docker.AgentTypeServer,
		},
		AutoRemove: false,
	}, a.composeProvider.GetSessionId())
}

func (a *Agent) StartAgentForSetVolume(ctx context.Context) error {
	agentMounts := make([]docker.ContainerMount, 0)
	agentMounts = append(agentMounts, docker.BindMount("/var/run/docker.sock", "/var/run/docker.sock"))
	agentMounts = append(agentMounts, docker.BindMount(a.composeProvider.GetContextPathForMount(), common.AgentContextPath))
	agentMounts = append(agentMounts, docker.VolumeMount(common.SystemLogVolumeName+"_"+a.GetSessionId(), common.AgentLogPath))
	cmd := make([]string, 0)
	cmd = append(cmd, "prepareVolume", "--fromConfigJson", config.GetConfigJson())
	for _, volume := range a.composeProvider.GetConfig().Volumes {
		volumeId := volume.Name + "_" + a.composeProvider.GetSessionId()
		agentMounts = append(agentMounts, docker.VolumeMount(volumeId, docker.ContainerMountTarget(common.AgentVolumePath+volume.Name)))
	}
	containerName := common.ContainerNamePrefix + "agent_volume_" + a.composeProvider.GetSessionId()
	return a.runAndGetAgentError(ctx, docker.ContainerRequest{
		Image: config.ComposeConfig.Image.Agent,
		Name:  common.ContainerNamePrefix + "agent_volume_" + a.composeProvider.GetSessionId(),
		Env: map[string]string{
			common.LabelSessionID:     a.composeProvider.GetSessionId(),
			common.EnvHostContextPath: a.composeProvider.GetContextPathForMount(),
			common.TpcDebug:           os.Getenv(common.TpcDebug),
			common.TpcName:            containerName,
		},
		Mounts: agentMounts,
		Cmd:    cmd,
		Labels: map[string]string{
			docker.AgentType: docker.AgentTypeVolume,
		},
	}, true)
}
func (a *Agent) StartAgentForSetVolumeGroup(ctx context.Context, selectGroupIndex int) error {
	agentMounts := make([]docker.ContainerMount, 0)
	agentMounts = append(agentMounts, docker.BindMount("/var/run/docker.sock", "/var/run/docker.sock"))
	agentMounts = append(agentMounts, docker.BindMount(a.composeProvider.GetContextPathForMount(), common.AgentContextPath))
	agentMounts = append(agentMounts, docker.VolumeMount(common.SystemLogVolumeName+"_"+a.GetSessionId(), common.AgentLogPath))
	cmd := make([]string, 0)
	cmd = append(cmd, "prepareVolumeGroup")
	cmd = append(cmd, "-s")
	cmd = append(cmd, strconv.Itoa(selectGroupIndex))
	cmd = append(cmd, "--fromConfigJson")
	cmd = append(cmd, config.GetConfigJson())
	for _, volume := range a.composeProvider.GetConfig().VolumeGroups[selectGroupIndex].Volumes {
		volumeId := volume.Name + "_" + a.composeProvider.GetSessionId()
		agentMounts = append(agentMounts, docker.VolumeMount(volumeId, docker.ContainerMountTarget(common.AgentVolumePath+volume.Name)))
	}
	containerName := common.ContainerNamePrefix + "agent_volume_" + a.composeProvider.GetSessionId()
	return a.runAndGetAgentError(ctx, docker.ContainerRequest{
		Image: config.ComposeConfig.Image.Agent,
		Name:  containerName,
		Env: map[string]string{
			common.LabelSessionID:     a.composeProvider.GetSessionId(),
			common.EnvHostContextPath: a.composeProvider.GetContextPathForMount(),
			common.TpcDebug:           os.Getenv(common.TpcDebug),
			common.TpcName:            containerName,
		},
		Mounts: agentMounts,
		Cmd:    cmd,
		Labels: map[string]string{
			docker.AgentType: docker.AgentTypeVolume,
		},
	}, true)
}

func (a *Agent) StartAgentForClean(ctx context.Context) error {
	containerName := common.ContainerNamePrefix + "agent_clean_" + a.composeProvider.GetSessionId()
	return a.runAndGetAgentError(ctx, docker.ContainerRequest{
		Image:  config.ComposeConfig.Image.Agent,
		Mounts: docker.Mounts(docker.BindMount("/var/run/docker.sock", "/var/run/docker.sock")),
		Name:   containerName,
		Env: map[string]string{
			common.LabelSessionID: a.composeProvider.GetSessionId(),
			common.TpcDebug:       os.Getenv(common.TpcDebug),
			common.TpcName:        containerName,
		},
		Cmd: []string{"clean", "--fromConfigJson", config.GetConfigJson()},
		Labels: map[string]string{
			docker.AgentType: docker.AgentTypeCleaner,
		},
		AutoRemove: true, //clean must set auto remove,agent cannot remove clean container
	}, true)
}

func (a *Agent) StartAgentForSwitchData(ctx context.Context, selectGroupIndex int) error {
	agentMounts := make([]docker.ContainerMount, 0)
	agentMounts = append(agentMounts, docker.BindMount("/var/run/docker.sock", "/var/run/docker.sock"))
	agentMounts = append(agentMounts, docker.BindMount(a.composeProvider.GetContextPathForMount(), common.AgentContextPath))
	agentMounts = append(agentMounts, docker.VolumeMount(common.SystemLogVolumeName+"_"+a.GetSessionId(), common.AgentLogPath))
	cmd := make([]string, 0)
	cmd = append(cmd, "switch")
	cmd = append(cmd, "-s")
	cmd = append(cmd, strconv.Itoa(selectGroupIndex))
	cmd = append(cmd, "--fromConfigJson")
	cmd = append(cmd, config.GetConfigJson())
	containerName := common.ContainerNamePrefix + "agent_switch_" + a.composeProvider.GetSessionId()
	return a.runAndGetAgentError(ctx, docker.ContainerRequest{
		Image: config.ComposeConfig.Image.Agent,
		Name:  containerName,
		Env: map[string]string{
			common.LabelSessionID:     a.composeProvider.GetSessionId(),
			common.EnvHostContextPath: a.composeProvider.GetContextPathForMount(),
			common.TpcDebug:           os.Getenv(common.TpcDebug),
			common.TpcName:            containerName,
		},
		Mounts: agentMounts,
		Cmd:    cmd,
		Labels: map[string]string{
			docker.AgentType: docker.AgentTypeSwitchData,
		},
	}, true)
}
func (a *Agent) startAgentForIngressSetVolume(ctx context.Context, volumeId string, servicePortInfo map[string]string) error {
	agentMounts := make([]docker.ContainerMount, 0)
	agentMounts = append(agentMounts, docker.BindMount("/var/run/docker.sock", "/var/run/docker.sock"))
	agentMounts = append(agentMounts, docker.VolumeMount(volumeId, docker.ContainerMountTarget(filepath.Join(common.AgentVolumePath, common.IngressVolumeName))))
	agentMounts = append(agentMounts, docker.VolumeMount(common.SystemLogVolumeName+"_"+a.GetSessionId(), common.AgentLogPath))
	cmd := make([]string, 0)
	cmd = append(cmd, "prepareIngressVolume")
	for serviceName, portMapping := range servicePortInfo {
		cmd = append(cmd, "-p")
		cmd = append(cmd, serviceName+"="+portMapping)
		cmd = append(cmd, "--fromConfigJson")
		cmd = append(cmd, config.GetConfigJson())
	}
	containerName := common.ContainerNamePrefix + "agent_ingress_volume" + a.composeProvider.GetSessionId()
	return a.runAndGetAgentError(ctx, docker.ContainerRequest{
		Image: config.ComposeConfig.Image.Agent,
		Name:  common.ContainerNamePrefix + "agent_ingress_volume" + a.composeProvider.GetSessionId(),
		Env: map[string]string{
			common.LabelSessionID:     a.composeProvider.GetSessionId(),
			common.EnvHostContextPath: a.composeProvider.GetContextPathForMount(),
			common.TpcDebug:           os.Getenv(common.TpcDebug),
			common.TpcName:            containerName,
		},
		Mounts: agentMounts,
		Cmd:    cmd,
		Labels: map[string]string{
			docker.AgentType: docker.AgentTypeIngressVolume,
		},
	}, true)
}

func (a *Agent) StartAgentForIngress(ctx context.Context, servicePortInfo map[string]string) (docker.Container, error) {
	volumeName := common.IngressVolumeName
	volumeId := volumeName + "_" + a.GetSessionId()
	containerName := common.ContainerNamePrefix + "agent_ingress_" + a.composeProvider.GetSessionId()
	// first remove ingress container
	ingressContainer, _ := a.composeProvider.GetDockerProvider().FindContainerByName(ctx, containerName)
	if ingressContainer != nil {
		_ = a.composeProvider.GetDockerProvider().RemoveContainer(ctx, ingressContainer.ID)
	}
	// then remove ingress volume
	_ = a.composeProvider.GetDockerProvider().RemoveVolume(ctx, volumeName, a.GetSessionId(), true)
	// create ingress volume
	_, err := a.composeProvider.GetDockerProvider().CreateVolume(ctx, volumeName, a.GetSessionId(), "")
	if err != nil {
		return nil, err
	}
	// prepare volume
	err = a.startAgentForIngressSetVolume(ctx, volumeId, servicePortInfo)
	if err != nil {
		return nil, err
	}
	// start ingress container
	exposePorts := make([]string, 0)
	for _, portInfo := range servicePortInfo {
		ports := strings.SplitN(portInfo, ":", 2)
		exposePorts = append(exposePorts, ports[1]+":"+ports[1])
	}
	container, err := a.composeProvider.GetDockerProvider().RunContainer(ctx, docker.ContainerRequest{
		Image:  config.ComposeConfig.Image.Ingress,
		Name:   containerName,
		Mounts: docker.Mounts(docker.VolumeMount(volumeId, "/etc/envoy")),
		Env: map[string]string{
			common.TpcDebug: os.Getenv(common.TpcDebug),
			common.TpcName:  containerName,
		},
		ExposedPorts: exposePorts,
		Labels: map[string]string{
			docker.AgentType: docker.AgentTypeIngress,
		},
		Networks: []string{a.composeProvider.GetConfig().Network},
	}, a.GetSessionId())
	if err != nil {
		return nil, err
	}
	collectLogs(container)
	return container, nil
}

// must use waitingFor exit
func (a *Agent) runAndGetAgentError(ctx context.Context, containerRequest docker.ContainerRequest, remove bool) error {
	containerRequest.WaitingFor = wait.ForExit()
	container, err := a.composeProvider.GetDockerProvider().CreateContainerAutoLabel(ctx, containerRequest, a.composeProvider.GetSessionId())
	if err != nil {
		return err
	}
	if err := container.Start(ctx, containerRequest); err != nil {
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
		return errors.New("agent tool exit with error code, container id: " + container.GetContainerID())
	}
	return nil
}
