package compose

import (
	"context"
	"github.com/docker/docker/api/types"
	"podcompose/docker"
)

type VolumeGroups struct {
	volumeGroupConfigs []*VolumeGroupConfig
	dockerProvider     *docker.DockerProvider
}

func NewVolumeGroups(volumes []*VolumeGroupConfig, dockerProvider *docker.DockerProvider) *VolumeGroups {
	return &VolumeGroups{
		volumeGroupConfigs: volumes,
		dockerProvider:     dockerProvider,
	}
}
func (v *VolumeGroups) createVolume(ctx context.Context, sessionId string, volumeName string) (types.Volume, error) {
	return v.dockerProvider.CreateVolume(ctx, volumeName, sessionId, "")
}
func (v *VolumeGroups) createVolumes(ctx context.Context, sessionId string, volumes []*VolumeConfig) error {
	for _, volume := range volumes {
		_, err := v.dockerProvider.CreateVolume(ctx, volume.Name, sessionId, "")
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *VolumeGroups) createVolumesWithGroup(ctx context.Context, sessionId string, volumeGroup *VolumeGroupConfig) error {
	for _, volume := range volumeGroup.Volumes {
		_, err := v.dockerProvider.CreateVolume(ctx, volume.Name, sessionId, volumeGroup.Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func (v *VolumeGroups) recreateVolumesWithGroup(ctx context.Context, volumeGroup *VolumeGroupConfig, sessionId string) error {
	for _, volume := range volumeGroup.Volumes {
		err := v.dockerProvider.RemoveVolume(ctx, volume.Name, sessionId, true)
		if err != nil {
			return err
		}
		_, err = v.dockerProvider.CreateVolume(ctx, volume.Name, sessionId, volumeGroup.Name)
		if err != nil {
			return err
		}
	}
	return nil
}
