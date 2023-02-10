package compose

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-connections/nat"
	"podcompose/common"
	"podcompose/docker"
	"podcompose/docker/wait"
	"podcompose/event"
	"sync"
	"time"
)

type PodCompose struct {
	sessionId      string
	orderPods      []map[string]*PodConfig
	network        string
	dockerProvider *docker.DockerProvider
	pods           map[string]*PodConfig
	observe        *Observe
}

func NewPodCompose(sessionID string, pods []*PodConfig, network string, dockerProvider *docker.DockerProvider) (*PodCompose, error) {
	podMap := make(map[string]*PodConfig)
	for _, pod := range pods {
		podMap[pod.Name] = pod
	}
	if floors, err := BuildDependFloors(pods); err != nil {
		return nil, err
	} else {
		return &PodCompose{
			orderPods:      floors.GetStartOrder(),
			network:        network,
			dockerProvider: dockerProvider,
			sessionId:      sessionID,
			pods:           podMap,
			observe:        nil,
		}, nil
	}
}

func (p *PodCompose) start(ctx context.Context) error {
	p.observe = &Observe{}
	p.observe.Start(p.dockerProvider)
	for _, pods := range p.orderPods {
		if err := p.concurrencyCreatePods(ctx, pods); err != nil {
			return err
		}
	}
	return nil
}

func (p *PodCompose) concurrencyCreatePods(ctx context.Context, pods map[string]*PodConfig) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errorChannel := make(chan error, len(pods))
	var wg sync.WaitGroup
	for _, pod := range pods {
		wg.Add(1)
		_pod := pod
		go func() {
			err := p.createPod(ctx, _pod)
			wg.Done()
			if err != nil {
				errorChannel <- err
				cancel()
			}
		}()
	}
	wg.Wait()
	select {
	case composeError := <-errorChannel:
		return composeError
	default:
		return nil
	}
}

func (p *PodCompose) createPod(ctx context.Context, pod *PodConfig) error {
	event.Publish(ctx, &event.PodEventData{
		TracingData: event.TracingData{
			PodName: pod.Name,
		},
		Type: event.PodEventStartType,
		Name: pod.Name,
	})
	containers := make([]docker.Container, 0)
	// create pause container
	pauseContainer, err := p.dockerProvider.RunContainer(ctx, docker.ContainerRequest{
		Name: common.ContainerNamePrefix + pod.Name + "_pause_" + p.sessionId,
		NetworkAliases: map[string][]string{
			p.network: {pod.Name},
		},
		Image:    common.ImagePause,
		Networks: []string{p.dockerProvider.GetDefaultNetwork(), p.network},
		DNS:      pod.Dns,
		CapAdd:   []string{"NET_ADMIN", "NET_RAW"},
		Labels: map[string]string{
			common.LabelPodName:       pod.Name,
			common.LabelContainerName: "pause",
		},
		AutoRemove: true,
	}, p.sessionId)
	if err != nil {
		return err
	}
	containers = append(containers, pauseContainer)
	for _, c := range pod.InitContainers {
		createContainer, req, err := p.runContainer(pod.Name, true, ctx, c, pauseContainer.GetContainerID())
		if err := createContainer.Start(ctx, req); err != nil {
			return err
		}
		if err != nil {
			return err
		}
		containers = append(containers, createContainer)
	}
	for _, c := range pod.Containers {
		createContainer, req, err := p.runContainer(pod.Name, false, ctx, c, pauseContainer.GetContainerID())
		if err != nil {
			return err
		}
		if err := createContainer.Start(ctx, req); err != nil {
			return err
		}
		containers = append(containers, createContainer)
	}
	for _, c := range containers {
		p.observe.observeContainerId(c.GetContainerID())
	}
	event.Publish(ctx, &event.PodEventData{
		TracingData: event.TracingData{
			PodName: pod.Name,
		},
		Type: event.PodEventReadyType,
		Name: pod.Name,
	})
	return nil
}

func (p *PodCompose) createWaitingFor(isInit bool, c *ContainerConfig) wait.Strategy {
	var waitingFor wait.Strategy
	if isInit {
		exitCode := 0
		waitingFor = wait.ForExit().
			WithExitCode(&exitCode).
			WithPollInterval(1 * time.Second).
			WithExitTimeout(common.InitExitTimeOut * time.Millisecond)
	} else {
		if c.WaitingFor == nil {
			return nil
		}
		strategies := make([]wait.Strategy, 0)
		if c.WaitingFor.HttpGet != nil {
			strategies = append(strategies, wait.ForHTTP(c.WaitingFor.HttpGet.Path).
				WithPort(nat.Port(fmt.Sprintf("%d%s", c.WaitingFor.HttpGet.Port, "/tcp"))).
				WithMethod(c.WaitingFor.HttpGet.Method).
				WithPollInterval(time.Duration(c.WaitingFor.PeriodSeconds)*time.Millisecond).
				WithStartupTimeout(time.Duration(c.WaitingFor.InitialDelaySeconds)*time.Millisecond))
		}
		if c.WaitingFor.TcpSocket != nil {
			strategies = append(strategies, wait.ForListeningPort(nat.Port(fmt.Sprintf("%d%s", c.WaitingFor.HttpGet.Port, "/tcp"))).
				WithPollInterval(time.Duration(c.WaitingFor.PeriodSeconds)*time.Millisecond).
				WithStartupTimeout(time.Duration(c.WaitingFor.InitialDelaySeconds)*time.Millisecond))
		}
		if len(strategies) != 0 {
			waitingFor = wait.ForAll(strategies...)
		} else {
			waitingFor = nil
		}
	}
	return waitingFor
}

func (p *PodCompose) runContainer(podName string, isInit bool, ctx context.Context, c *ContainerConfig, pauseId string) (docker.Container, docker.ContainerRequest, error) {
	containerMounts := make([]docker.ContainerMount, 0)
	for _, vm := range c.VolumeMounts {
		containerMount := docker.VolumeMount(vm.Name+"_"+p.sessionId, docker.ContainerMountTarget(vm.MountPath))
		containerMounts = append(containerMounts, containerMount)
	}
	var capAdd strslice.StrSlice
	var capDrop strslice.StrSlice
	if c.Cap != nil {
		capAdd = c.Cap.Add
		capDrop = c.Cap.Drop
	}
	req := docker.ContainerRequest{
		Name:            common.ContainerNamePrefix + podName + "_" + c.Name + "_" + p.sessionId,
		Image:           c.Image,
		Cmd:             c.Command,
		Privileged:      c.Privileged,
		AlwaysPullImage: c.AlwaysPullImage,
		NetworkMode:     container.NetworkMode("container:" + pauseId),
		Mounts:          containerMounts,
		CapAdd:          capAdd,
		CapDrop:         capDrop,
		User:            c.User,
		Env:             c.Env,
		WaitingFor:      p.createWaitingFor(isInit, c),
		Labels: map[string]string{
			common.LabelPodName:       podName,
			common.LabelContainerName: c.Name,
		},
	}
	runContainer, err := p.dockerProvider.RunContainer(ctx, req, p.sessionId)
	if err != nil {
		return nil, docker.ContainerRequest{}, err
	}
	return runContainer, req, nil
}

func (p *PodCompose) foundContainerWithPods(ctx context.Context, pods map[string]*PodConfig) ([]types.Container, error) {
	containers, err := p.dockerProvider.FindContainers(ctx, p.sessionId)
	if err != nil {
		return nil, err
	}
	result := make([]types.Container, 0)
	for _, c := range containers {
		if _, ok := pods[c.Labels[common.LabelPodName]]; ok {
			result = append(result, c)
		}
	}
	return result, nil
}

func (p *PodCompose) RestartPods(ctx context.Context, pods []string, beforeStart func() error) error {
	needRestartPods := p.findWhoDependPods(pods, make(map[string]*PodConfig))
	containers, err := p.foundContainerWithPods(ctx, needRestartPods)
	if err != nil {
		return err
	}
	for _, c := range containers {
		err := p.dockerProvider.RemoveContainer(ctx, c.ID)
		if err != nil {
			return err
		}
	}
	err = beforeStart()
	if err != nil {
		return err
	}
	for _, orderPod := range p.orderPods {
		needConcurrencyCreatePods := make(map[string]*PodConfig)
		for _, restartPod := range needRestartPods {
			if _, ok := orderPod[restartPod.Name]; ok {
				needConcurrencyCreatePods[restartPod.Name] = restartPod
			}
		}
		if len(needConcurrencyCreatePods) > 0 {
			err := p.concurrencyCreatePods(ctx, needConcurrencyCreatePods)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *PodCompose) findWhoDependPods(podNames []string, depends map[string]*PodConfig) map[string]*PodConfig {
	size := len(depends)
	for _, podName := range podNames {
		depends[podName] = p.pods[podName]
		for _, pod := range p.pods {
			for _, dependName := range pod.Depends {
				if dependName == podName {
					depends[pod.Name] = p.pods[pod.Name]
					break
				}
			}
		}
	}
	if len(depends) == size {
		return depends
	}
	dependPodNames := make([]string, 0)
	for _, dependPod := range depends {
		dependPodNames = append(dependPodNames, dependPod.Name)
	}
	return p.findWhoDependPods(dependPodNames, depends)
}

func (p *PodCompose) findPodsWhoUsedVolumes(volumeNames []string) []*PodConfig {
	pods := make([]*PodConfig, 0)
	nameMap := make(map[string]string)
	for _, vn := range volumeNames {
		nameMap[vn] = vn
	}
	for _, pod := range p.pods {
		isBreak := false
		for _, c := range pod.Containers {
			if isBreak {
				break
			}
			for _, vm := range c.VolumeMounts {
				if _, ok := nameMap[vm.Name]; ok {
					pods = append(pods, pod)
					isBreak = true
					break
				}
			}
		}
		for _, c := range pod.InitContainers {
			if isBreak {
				break
			}
			for _, vm := range c.VolumeMounts {
				if _, ok := nameMap[vm.Name]; ok {
					pods = append(pods, pod)
					isBreak = true
					break
				}
			}
		}
	}
	return pods
}
