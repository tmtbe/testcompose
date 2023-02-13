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

func (v *Volume) copyDataToVolumes(nameAndData map[string]string) error {
	for _, v := range v.Compose.GetConfig().Volumes {
		if selectData, ok := nameAndData[v.Name]; ok {
			if v.SwitchData != nil {
				hostPath, ok := v.SwitchData[selectData]
				if !ok {
					return errors.Errorf("select data %s is not exist", selectData)
				}
				sourcePath := filepath.Join(common.AgentContextPath, hostPath)
				targetPath := filepath.Join(common.AgentVolumePath, v.Name)
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
		}
	}
	return nil
}
