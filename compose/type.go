package compose

import (
	"fmt"
	"github.com/go-playground/validator/v10"
	"github.com/pkg/errors"
	"os"
	"path/filepath"
)

type ComposeConfig struct {
	Version      string             `json:"version" yaml:"version" validate:"required"`
	SessionId    string             `json:"sessionId,omitempty" yaml:"sessionId,omitempty"`
	Network      string             `json:"network,omitempty" yaml:"network,omitempty"`
	TaskGroups   TaskGroups         `json:"taskGroups,omitempty" yaml:"taskGroups,omitempty" validate:"omitempty,dive"`
	Pods         []*PodConfig       `json:"pods,omitempty" yaml:"pods,omitempty" validate:"omitempty,dive"`
	VolumeGroups VolumeGroupConfigs `json:"volumeGroups,omitempty" yaml:"volumeGroups,omitempty" validate:"omitempty,dive"`
	Volumes      []*VolumeConfig    `json:"volumes,omitempty" yaml:"volumes,omitempty" validate:"omitempty,dive"`
}
type TaskGroups []*TaskGroup
type TaskGroup struct {
	Name  string             `json:"name" yaml:"name" validate:"required"`
	Event string             `json:"event" yaml:"event"`
	Tasks []*ContainerConfig `json:"tasks" yaml:"tasks" validate:"omitempty,dive"`
}

func (t TaskGroups) GetTaskGroupFromName(name string) *TaskGroup {
	for _, taskGroup := range t {
		if taskGroup.Name == name {
			return taskGroup
		}
	}
	return nil
}
func (t TaskGroups) GetTaskGroupFromEvent(event string) []*TaskGroup {
	result := make([]*TaskGroup, 0)
	for _, taskGroup := range t {
		if taskGroup.Event == event {
			result = append(result, taskGroup)
		}
	}
	return result
}

func (c *ComposeConfig) GetNetworkName() string {
	if c.Network == "" {
		return "PodTestComposeNetwork_" + c.SessionId
	} else {
		return c.Network
	}
}

func (c *ComposeConfig) check(contextPath string) error {
	validate := validator.New()
	err := validate.Struct(c)
	if err != nil {
		return err
	}
	if c.Version != "1" {
		return errors.New("version must be 1")
	}
	if c.SessionId == "" {
		return errors.New("not init session id")
	}
	needVolumeMap := make(map[string]string)
	for _, taskGroup := range c.TaskGroups {
		for _, task := range taskGroup.Tasks {
			for _, vm := range task.VolumeMounts {
				needVolumeMap[vm.Name] = vm.Name
			}
		}
	}
	podMap := make(map[string]string)
	for _, pod := range c.Pods {
		if _, ok := podMap[pod.Name]; ok {
			return errors.Errorf("duplicate pod name:%s", pod.Name)
		}
		podMap[pod.Name] = pod.Name
		if err := pod.check(c); err != nil {
			return err
		}
		for _, c := range pod.InitContainers {
			for _, vm := range c.VolumeMounts {
				needVolumeMap[vm.Name] = vm.Name
			}
		}
		for _, c := range pod.Containers {
			for _, vm := range c.VolumeMounts {
				needVolumeMap[vm.Name] = vm.Name
			}
		}
	}
	for _, v := range c.Volumes {
		delete(needVolumeMap, v.Name)
	}
	for _, vg := range c.VolumeGroups {
		volumeCheck := make(map[string]bool)
		for name := range needVolumeMap {
			volumeCheck[name] = false
		}
		for _, v := range vg.Volumes {
			_, ok := volumeCheck[v.Name]
			if ok {
				volumeCheck[v.Name] = true
			} else {
				return errors.New(fmt.Sprintf("volumeGroup name:%s, %s may be not need", vg.Name, v.Name))
			}
			err := v.check(contextPath)
			if err != nil {
				return err
			}
		}
		for name, check := range volumeCheck {
			if !check {
				return errors.New(fmt.Sprintf("can not found volume %s", name))
			}
		}
	}
	return nil
}

type VolumeGroupConfigs []*VolumeGroupConfig

func (v VolumeGroupConfigs) GetGroup(name string) (*VolumeGroupConfig, int) {
	var selectVolumeGroup *VolumeGroupConfig
	var selectIndex int
	for index, volumeGroup := range v {
		if volumeGroup.Name == name {
			selectVolumeGroup = volumeGroup
			selectIndex = index
			break
		}
	}
	return selectVolumeGroup, selectIndex
}

type VolumeGroupConfig struct {
	Name    string          `json:"name" yaml:"name" validate:"required"`
	Volumes []*VolumeConfig `json:"volumes" validate:"omitempty,dive"`
}

type VolumeConfig struct {
	Name string `json:"name" yaml:"name" validate:"required"`
	Path string `json:"path" yaml:"path"`
}

func (v *VolumeConfig) check(contextPath string) error {
	if v.Name == "" {
		return errors.New("volume name must be set")
	}
	if v.Path != "" {
		fileName := filepath.Join(contextPath, v.Path)
		_, err := os.Stat(fileName)
		if err != nil {
			return err
		}
	}
	return nil
}

type PodConfig struct {
	Name           string             `json:"name" yaml:"name" validate:"required"`
	InitContainers []*ContainerConfig `json:"initContainers,omitempty" yaml:"initContainers,omitempty" validate:"omitempty,dive"`
	Dns            []string           `json:"dns,omitempty" yaml:"dns,omitempty"`
	Containers     []*ContainerConfig `json:"containers" yaml:"containers" validate:"omitempty,dive"`
	Depends        []string           `json:"depends,omitempty" yaml:"depends,omitempty"`
}

func (p *PodConfig) check(cc *ComposeConfig) error {
	if p.Name == "" {
		return errors.New("pod name must be set")
	}
	for _, initContainer := range p.InitContainers {
		if err := initContainer.check(cc); err != nil {
			return err
		}
	}
	for _, container := range p.Containers {
		if err := container.check(cc); err != nil {
			return err
		}
	}
	podsMap := make(map[string]string)
	for _, pod := range cc.Pods {
		podsMap[pod.Name] = pod.Name
	}
	for _, depend := range p.Depends {
		if depend == p.Name {
			return errors.Errorf("%s depend:%s cannot rely on itself", p.Name, depend)
		}
		if _, ok := podsMap[depend]; !ok {
			return errors.Errorf("%s depend:%s not found in pods", p.Name, depend)
		}
	}
	return nil
}

type ContainerConfig struct {
	Name            string               `json:"name" yaml:"name" validate:"required"`
	Image           string               `json:"image" yaml:"image" validate:"required"`
	Privileged      bool                 `json:"privileged,omitempty" yaml:"privileged,omitempty"`
	AlwaysPullImage bool                 `json:"alwaysPullImage,omitempty" yaml:"alwaysPullImage,omitempty"`
	BindMounts      []*BindMountConfig   `json:"bindMounts,omitempty" yaml:"bindMounts,omitempty" validate:"omitempty,dive"`
	VolumeMounts    []*VolumeMountConfig `json:"volumeMounts,omitempty" yaml:"volumeMounts,omitempty" validate:"omitempty,dive"`
	Env             map[string]string    `json:"env,omitempty" yaml:"env,omitempty"`
	Command         []string             `json:"command,omitempty" yaml:"command,omitempty"`
	Cap             *CapConfig           `json:"cap,omitempty" yaml:"cap,omitempty"`
	WaitingFor      *WaitingForConfig    `json:"waitingFor,omitempty" yaml:"waitingFor,omitempty" validate:"omitempty,dive"`
	User            string               `json:"user,omitempty" yaml:"user,omitempty"`
	WorkingDir      string               `json:"workingDir,omitempty" yaml:"workingDir,omitempty"`
}

func (cc *ContainerConfig) check(c *ComposeConfig) error {
	if cc == nil {
		return nil
	}
	return cc.WaitingFor.check()
}

type VolumeMountConfig struct {
	Name      string `json:"name" yaml:"name" validate:"required"`
	MountPath string `json:"mountPath" yaml:"mountPath" validate:"required"`
}

type BindMountConfig struct {
	HostPath  string `json:"hostPath" yaml:"hostPath" validate:"required"`
	MountPath string `json:"mountPath" yaml:"mountPath" validate:"required"`
}

type CapConfig struct {
	Add  []string `json:"add" yaml:"add"`
	Drop []string `json:"drop" yaml:"drop"`
}
type WaitingForConfig struct {
	HttpGet             *HttpGetConfig   `json:"httpGet" json:"httpGet" validate:"omitempty,dive"`
	TcpSocket           *TcpSocketConfig `json:"tcpSocket" json:"tcpSocket" validate:"omitempty,dive"`
	InitialDelaySeconds int              `json:"initialDelaySeconds" yaml:"initialDelaySeconds"`
	PeriodSeconds       int              `json:"periodSeconds" yaml:"periodSeconds"`
}

func (wf *WaitingForConfig) check() error {
	if wf == nil {
		return nil
	}
	if wf.PeriodSeconds == 0 {
		wf.PeriodSeconds = 100
	}
	return nil
}

type HttpGetConfig struct {
	Method string `json:"method" yaml:"method" validate:"required"`
	Path   string `json:"path" yaml:"path" validate:"required"`
	Port   int    `json:"port" yaml:"port" validate:"required"`
}
type TcpSocketConfig struct {
	Port int `json:"port" yaml:"port" validate:"required"`
}
