package compose

import (
	"context"
	"podcompose/docker"
)

type Volume struct {
	volumes        []*VolumeConfig
	dockerProvider *docker.DockerProvider
}

func NewVolumes(volumes []*VolumeConfig, dockerProvider *docker.DockerProvider) *Volume {
	return &Volume{
		volumes:        volumes,
		dockerProvider: dockerProvider,
	}
}

func (v *Volume) createVolumes(ctx context.Context, sessionId string) error {
	for _, volume := range v.volumes {
		_, err := v.dockerProvider.CreateVolume(ctx, volume.Name+"_"+sessionId, sessionId)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *Volume) reCreateVolumes(ctx context.Context, names []string, sessionId string) error {
	err := v.dockerProvider.RemoveVolumes(ctx, names, true)
	if err != nil {
		return err
	}
	for _, name := range names {
		_, err := v.dockerProvider.CreateVolume(ctx, name, sessionId)
		if err != nil {
			return err
		}
	}
	return nil
}
