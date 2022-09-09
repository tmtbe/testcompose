package main

import (
	"context"
	"go.uber.org/zap"
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
}

func (p *Plist) ps(ctx context.Context) (map[string]PlistStruct, error) {
	containers, err := p.dockerProvider.FindAllPodContainers(ctx)
	if err != nil {
		return nil, err
	}
	plists := make(map[string]PlistStruct)
	for _, c := range containers {
		if plist, ok := plists[c.Labels[docker.ComposeSessionID]]; ok {
			alive := false
			if c.Labels[docker.AgentType] == "agent" && c.Status == "" {
				alive = true
			}
			if !plist.Alive && alive {
				plists[c.Labels[docker.ComposeSessionID]] = PlistStruct{
					Name:  c.Labels[docker.ComposeSessionID],
					Alive: true,
				}
			}
		} else {
			alive := false
			if c.Labels[docker.AgentType] == "agent" && c.Status == "" {
				alive = true
			}
			plists[c.Labels[docker.ComposeSessionID]] = PlistStruct{
				Name:  c.Labels[docker.ComposeSessionID],
				Alive: alive,
			}
		}
	}
	return plists, nil
}

func (p *PlistCmd) Ps() error {
	ctx := context.Background()
	ps, err := p.ps(ctx)
	for _, p := range ps {
		zap.L().Info(p.Name)
	}
	return err
}
