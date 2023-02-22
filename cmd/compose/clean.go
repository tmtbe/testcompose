package main

import (
	"context"
	"go.uber.org/zap"
	"podcompose/docker"
)

type CleanCmd struct {
	dockerProvider *docker.DockerProvider
	plist          *Plist
}

func NewCleanCmd() (*CleanCmd, error) {
	dockerProvider, err := docker.NewDockerProvider()
	if err != nil {
		return nil, err
	}
	return &CleanCmd{
		dockerProvider: dockerProvider,
		plist:          NewPlist(dockerProvider),
	}, nil
}
func (c *CleanCmd) clean(allFlag bool) error {
	ctx := context.Background()
	protect := make(map[string]string)
	if !allFlag {
		ps, err := c.plist.ps(ctx)
		if err != nil {
			return err
		}
		for _, p := range ps {
			protect[p.Name] = p.Name
		}
	}
	// clean container
	containers, err := c.dockerProvider.FindAllPodContainers(ctx)
	if err == nil {
		for _, container := range containers {
			if _, ok := protect[container.Labels[docker.ComposeSessionID]]; !ok {
				zap.L().Sugar().Infof("remove container:%s", container.ID)
				err := c.dockerProvider.RemoveContainer(ctx, container.ID)
				if err != nil {
					zap.L().Sugar().Error(err)
				}
			}
		}
	} else {
		zap.L().Sugar().Error(err)
	}
	// clean volume
	volumes, err := c.dockerProvider.FindAllVolumes(ctx)
	if err == nil {
		for _, volume := range volumes {
			if _, ok := protect[volume.Labels[docker.ComposeSessionID]]; !ok {
				zap.L().Sugar().Infof("remove volume:%s", volume.Name)
				err := c.dockerProvider.RemoveVolume(ctx, volume.Name, "", true)
				if err != nil {
					zap.L().Sugar().Error(err)
				}
			}
		}
	} else {
		zap.L().Sugar().Error(err)
	}
	// clean network
	networks, err := c.dockerProvider.FindAllNetworks(ctx)
	if err == nil {
		for _, network := range networks {
			if _, ok := protect[network.Labels[docker.ComposeSessionID]]; !ok {
				zap.L().Sugar().Infof("remove volume:%s", network.Name)
				err := c.dockerProvider.RemoveNetwork(ctx, network.ID)
				if err != nil {
					zap.L().Sugar().Error(err)
				}
			}
		}
	} else {
		zap.L().Sugar().Error(err)
	}
	return nil
}
