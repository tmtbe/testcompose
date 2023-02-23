package compose

import (
	"fmt"
	"github.com/pkg/errors"
	"os"
	"path/filepath"
)

type ComposeConfig struct {
	Version      string             `json:"version" yaml:"version"`
	SessionId    string             `json:"sessionId" yaml:"sessionId"`
	Network      string             `json:"network" yaml:"network"`
	TaskGroups   TaskGroups         `json:"taskGroups" yaml:"taskGroups"`
	Pods         []*PodConfig       `json:"pods" yaml:"pods"`
	VolumeGroups VolumeGroupConfigs `json:"volumeGroups" yaml:"volumeGroups"`
	Volumes      []*VolumeConfig    `json:"volumes" yaml:"volumes"`
}
type TaskGroups []*TaskGroup
type TaskGroup struct {
	Name  string             `json:"name" yaml:"name"`
	Event string             `json:"event" yaml:"event"`
	Tasks []*ContainerConfig `json:"tasks" yaml:"tasks"`
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
	Name    string          `json:"name" yaml:"name"`
	Volumes []*VolumeConfig `json:"volumes"`
}

type VolumeConfig struct {
	Name string `json:"name" yaml:"name"`
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
	Name           string             `json:"name" yaml:"name"`
	InitContainers []*ContainerConfig `json:"initContainers" yaml:"initContainers"`
	Dns            []string           `json:"dns" yaml:"dns"`
	Containers     []*ContainerConfig `json:"containers" yaml:"containers"`
	Depends        []string           `json:"depends" yaml:"depends"`
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
	Name            string               `json:"name" yaml:"name"`
	Image           string               `json:"image" yaml:"image"`
	Privileged      bool                 `json:"privileged" yaml:"privileged"`
	AlwaysPullImage bool                 `json:"alwaysPullImage" yaml:"alwaysPullImage"`
	VolumeMounts    []*VolumeMountConfig `json:"volumeMounts" yaml:"volumeMounts"`
	BindMounts      []*BindMountConfig   `json:"bindMounts" yaml:"bindMounts"`
	Env             map[string]string    `json:"env" yaml:"env"`
	Command         []string             `json:"command" yaml:"command"`
	Cap             *CapConfig           `json:"cap" yaml:"cap"`
	WaitingFor      *WaitingForConfig    `json:"waitingFor" yaml:"waitingFor"`
	User            string               `json:"user" yaml:"user"`
	WorkingDir      string               `json:"workingDir" yaml:"workingDir"`
}

func (cc *ContainerConfig) check(c *ComposeConfig) error {
	if cc == nil {
		return nil
	}
	return cc.WaitingFor.check()
}

type VolumeMountConfig struct {
	Name      string `json:"name" yaml:"name"`
	MountPath string `json:"mountPath" yaml:"mountPath"`
}

type BindMountConfig struct {
	HostPath  string `json:"hostPath" yaml:"hostPath"`
	MountPath string `json:"mountPath" yaml:"mountPath"`
}

type EnvConfig struct {
	Name  string `json:"name" yaml:"name"`
	Value string `json:"value" yaml:"value"`
}
type CapConfig struct {
	Add  []string `json:"add" yaml:"add"`
	Drop []string `json:"drop" yaml:"drop"`
}
type WaitingForConfig struct {
	HttpGet             *HttpGetConfig   `json:"httpGet" json:"httpGet"`
	TcpSocket           *TcpSocketConfig `json:"tcpSocket" json:"tcpSocket"`
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
	Method string `json:"method" yaml:"method"`
	Path   string `json:"path" yaml:"path"`
	Port   int    `json:"port" yaml:"port"`
}
type TcpSocketConfig struct {
	Port int `json:"port" yaml:"port"`
}
