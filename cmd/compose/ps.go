package main

import (
	"context"
	"fmt"
	"github.com/gosuri/uitable"
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
	Name  string
	Alive bool
	Port  string
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
			port := p.getPort(ctx, docker.NewDockerContainer(c.ID, c.Image, p.dockerProvider, c.Labels[docker.ComposeSessionID], nil))
			plists[c.Labels[docker.ComposeSessionID]] = PlistStruct{
				Name:  c.Labels[docker.ComposeSessionID],
				Alive: alive,
				Port:  port,
			}
		}
	}
	return plists, nil
}

func (p *Plist) getPort(ctx context.Context, container *docker.DockerContainer) string {
	ports, err := container.Ports(ctx)
	if err != nil {
		return ""
	}
	for _, portBinds := range ports {
		if len(portBinds) > 0 {
			return portBinds[0].HostPort
		}
	}
	return ""
}

func (p *PlistCmd) Ps() error {
	ctx := context.Background()
	ps, err := p.ps(ctx)
	table := uitable.New()
	table.MaxColWidth = 50
	table.AddRow("NAME", "ALIVE", "PORT")
	for _, p := range ps {
		table.AddRow(p.Name, p.Alive, p.Port)
	}
	fmt.Println(table)
	return err
}
