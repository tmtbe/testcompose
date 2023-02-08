package main

import (
	"context"
	"fmt"
	"github.com/docker/go-connections/nat"
	"github.com/gosuri/uitable"
	"podcompose/common"
	"podcompose/docker"
)

type PlistCmd struct {
	*Plist
}
type Plist struct {
	dockerProvider *docker.DockerProvider
}

func NewPlist(dockerProvider *docker.DockerProvider) *Plist {
	return &Plist{
		dockerProvider: dockerProvider,
	}
}
func NewPlistCmd() (*PlistCmd, error) {
	provider, err := docker.NewDockerProvider()
	if err != nil {
		return nil, err
	}
	return &PlistCmd{
		NewPlist(provider),
	}, nil
}

type PlistStruct struct {
	Name         string
	Alive        bool
	AgentPort    string
	EventBusPort string
}

func (p *Plist) ps(ctx context.Context) (map[string]PlistStruct, error) {
	containers, err := p.dockerProvider.FindAllPodContainers(ctx)
	if err != nil {
		return nil, err
	}
	plists := make(map[string]PlistStruct)
	for _, c := range containers {
		if c.Labels[docker.AgentType] == docker.AgentTypeServer {
			alive := false
			if c.State == "running" {
				alive = true
			}
			agentPort := p.getPort(ctx, docker.NewDockerContainer(c.ID, c.Image, p.dockerProvider, c.Labels[docker.ComposeSessionID], nil), common.ServerAgentPort)
			eventBusPort := p.getPort(ctx, docker.NewDockerContainer(c.ID, c.Image, p.dockerProvider, c.Labels[docker.ComposeSessionID], nil), common.ServerAgentPort)
			plists[c.Labels[docker.ComposeSessionID]] = PlistStruct{
				Name:         c.Labels[docker.ComposeSessionID],
				Alive:        alive,
				AgentPort:    agentPort,
				EventBusPort: eventBusPort,
			}
		}
	}
	return plists, nil
}

func (p *Plist) getPort(ctx context.Context, container *docker.DockerContainer, portName string) string {
	ports, err := container.Ports(ctx)
	if err != nil {
		return ""
	}
	natPort, _ := nat.NewPort("tcp", portName)
	for port, portBinds := range ports {
		if port == natPort {
			if len(portBinds) > 0 {
				return portBinds[0].HostPort
			}
		}
	}
	return ""
}

func (p *PlistCmd) Ps() error {
	ctx := context.Background()
	ps, err := p.ps(ctx)
	table := uitable.New()
	table.MaxColWidth = 50
	table.AddRow("NAME", "ALIVE", "AGENT_PORT", "EVENT_BUS_PORT")
	for _, p := range ps {
		table.AddRow(p.Name, p.Alive, p.AgentPort, p.EventBusPort)
	}
	fmt.Println(table)
	return err
}
