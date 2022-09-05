package compose

import (
	"context"
	"github.com/docker/docker/api/types"
	"podcompose/docker"
)

type Volume struct {
	volumes             []*VolumeConfig
	dockerProvider      *docker.DockerProvider
	createVolumesResult []*types.Volume
}

func NewVolumes(volumes []*VolumeConfig, dockerProvider *docker.DockerProvider) *Volume {
	return &Volume{
		volumes:             volumes,
		dockerProvider:      dockerProvider,
		createVolumesResult: make([]*types.Volume, 0),
	}
}

func (v *Volume) createVolumes(ctx context.Context, sessionId string) error {
	for _, volume := range v.volumes {
		createVolume, err := v.dockerProvider.CreateVolume(ctx, volume.Name, sessionId)
		if err != nil {
			return err
		}
		v.createVolumesResult = append(v.createVolumesResult, &createVolume)
	}
	return nil
}

func (v *Volume) clean(ctx context.Context) {
	for _, cvr := range v.createVolumesResult {
		_ = v.dockerProvider.RemoveVolume(ctx, cvr.Name, true)
	}
}
