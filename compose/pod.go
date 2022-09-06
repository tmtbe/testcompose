package compose

import (
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-connections/nat"
	"podcompose/docker"
	"podcompose/docker/wait"
	"sync"
	"time"
)

const (
	PauseImage      = "gcr.io/google_containers/pause:3.0"
	InitExitTimeOut = 60000
)

type PodCompose struct {
	sessionId      string
	orderPods      [][]*PodConfig
	network        string
	dockerProvider *docker.DockerProvider
	podContainers  map[string][]docker.Container
}

func NewPodCompose(sessionID string, pods []*PodConfig, network string, dockerProvider *docker.DockerProvider) (*PodCompose, error) {
	if floors, err := BuildDependFloors(pods); err != nil {
		return nil, err
	} else {
		return &PodCompose{
			orderPods:      floors.GetStartOrder(),
			network:        network,
			dockerProvider: dockerProvider,
			podContainers:  make(map[string][]docker.Container, 0),
			sessionId:      sessionID,
		}, nil
	}
}

func (p *PodCompose) clean(ctx context.Context) {
	for _, containers := range p.podContainers {
		for _, c := range containers {
			_ = c.Terminate(ctx)
		}
	}
}
func (p *PodCompose) start(ctx context.Context) error {
	for _, pods := range p.orderPods {
		return p.concurrencyCreatePods(ctx, pods)
	}
	return nil
}

func (p *PodCompose) concurrencyCreatePods(ctx context.Context, pods []*PodConfig) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	errorChannel := make(chan error, len(pods))
	var wg sync.WaitGroup
	for _, pod := range pods {
		wg.Add(1)
		go func() {
			err := p.createPod(ctx, pod)
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
	containers := make([]docker.Container, 0)
	// create pause container
	pauseContainer, err := p.dockerProvider.RunContainer(ctx, docker.ContainerRequest{
		Name: pod.Name + "_" + p.sessionId,
		NetworkAliases: map[string][]string{
			p.network: {pod.Name},
		},
		Image:      PauseImage,
		Networks:   []string{p.dockerProvider.GetDefaultNetwork(), p.network},
		DNS:        pod.Dns,
		CapAdd:     []string{"NET_ADMIN", "NET_RAW"},
		AutoRemove: true,
	}, p.sessionId)
	if err != nil {
		return err
	}
	containers = append(containers, pauseContainer)
	for _, c := range pod.InitContainers {
		createContainer, err := p.runContainer(pod.Name, true, ctx, c, pauseContainer.GetContainerID())
		if err != nil {
			return err
		}
		containers = append(containers, createContainer)
	}
	for _, c := range pod.Containers {
		createContainer, err := p.runContainer(pod.Name, false, ctx, c, pauseContainer.GetContainerID())
		if err != nil {
			return err
		}
		if err := createContainer.Start(ctx); err != nil {
			return err
		}
		containers = append(containers, createContainer)
	}
	p.podContainers[pod.Name] = containers
	return nil
}

func (p *PodCompose) createWaitingFor(isInit bool, c *ContainerConfig) wait.Strategy {
	var waitingFor wait.Strategy
	if isInit {
		exitCode := 0
		waitingFor = wait.ForExit().
			WithExitCode(&exitCode).
			WithPollInterval(1 * time.Second).
			WithExitTimeout(InitExitTimeOut * time.Millisecond)
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

func (p *PodCompose) runContainer(podName string, isInit bool, ctx context.Context, c *ContainerConfig, pauseId string) (docker.Container, error) {
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
	return p.dockerProvider.RunContainer(ctx, docker.ContainerRequest{
		Name:            podName + "_" + c.Name + "_" + p.sessionId,
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
		AutoRemove:      true,
	}, p.sessionId)
}
