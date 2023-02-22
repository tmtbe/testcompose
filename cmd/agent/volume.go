package main

import (
	"github.com/pkg/errors"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"podcompose/common"
	"podcompose/compose"
)

type Volume struct {
	*compose.Compose
}

func NewVolume(workspace string, sessionId string) (*Volume, error) {
	workspace, err := filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	configByte, err := os.ReadFile(filepath.Join(workspace, common.ConfigFileName))
	if err != nil {
		return nil, err
	}
	c, err := compose.NewCompose(configByte, sessionId, workspace, workspace)
	if err != nil {
		return nil, err
	}
	return &Volume{
		Compose: c,
	}, nil
}
func (v *Volume) copyDataToVolume(volumeConfigs []*compose.VolumeConfig) error {
	for _, volume := range volumeConfigs {
		hostPath := volume.Path
		sourcePath := filepath.Join(common.AgentContextPath, hostPath)
		targetPath := filepath.Join(common.AgentVolumePath, volume.Name)
		rd, err := ioutil.ReadDir(sourcePath)
		if err != nil {
			return err
		}
		for _, fi := range rd {
			stdout, err := exec.Command("cp", "-r", filepath.Join(sourcePath, fi.Name()), targetPath).CombinedOutput()
			if err != nil {
				return errors.Wrap(err, string(stdout))
			}
		}
	}
	return nil
}
func (v *Volume) copyDataToVolumeGroup(selectGroupIndex int) error {
	return v.copyDataToVolume(v.GetConfig().VolumeGroups[selectGroupIndex].Volumes)
}
