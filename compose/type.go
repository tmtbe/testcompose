package compose

import (
	"github.com/pkg/errors"
	"os"
	"path/filepath"
)

type ComposeConfig struct {
	Version   string          `json:"version" yaml:"version"`
	SessionId string          `json:"sessionId" yaml:"sessionId"`
	Network   string          `json:"network" yaml:"network"`
	Pods      []*PodConfig    `json:"pods" yaml:"pods"`
	Volumes   []*VolumeConfig `json:"volumes" yaml:"volumes"`
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
	podMap := make(map[string]string)
	for _, pod := range c.Pods {
		if _, ok := podMap[pod.Name]; ok {
			return errors.Errorf("duplicate pod name:%s", pod.Name)
		}
		podMap[pod.Name] = pod.Name
		if err := pod.check(c); err != nil {
			return err
		}
	}
	volumeMap := make(map[string]string)
	for _, v := range c.Volumes {
		if _, ok := volumeMap[v.Name]; ok {
			return errors.Errorf("duplicate volume name:%s", v.Name)
		}
		volumeMap[v.Name] = v.Name
		if err := v.check(contextPath); err != nil {
			return err
		}
	}
	return nil
}

type VolumeConfig struct {
	Name       string            `json:"name" yaml:"name"`
	EmptyDir   map[string]string `json:"emptyDir" yaml:"emptyDir"`
	SwitchData map[string]string `json:"switchData" yaml:"switchData"`
}

func (v *VolumeConfig) check(contextPath string) error {
	if v.Name == "" {
		return errors.New("volume name must be set")
	}
	if v.EmptyDir == nil && v.SwitchData == nil {
		return errors.New("volume emptyDir hostPath cannot be null at the same time")
	}
	if v.SwitchData != nil {
		_, ok := v.SwitchData["normal"]
		if !ok {
			return errors.Errorf("volume:[%s] not found \"normal\" host path", v.Name)
		}
		for _, path := range v.SwitchData {
			fileName := filepath.Join(contextPath, path)
			_, err := os.Stat(fileName)
			if err != nil {
				return err
			}
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
	Ports           []int                `json:"ports" yaml:"ports"`
	Privileged      bool                 `json:"privileged" yaml:"privileged"`
	AlwaysPullImage bool                 `json:"alwaysPullImage" yaml:"alwaysPullImage"`
	VolumeMounts    []*VolumeMountConfig `json:"volumeMounts" yaml:"volumeMounts"`
	Env             map[string]string    `json:"env" yaml:"env"`
	Command         []string             `json:"command" yaml:"command"`
	Cap             *CapConfig           `json:"cap" yaml:"cap"`
	WaitingFor      *WaitingForConfig    `json:"waitingFor" yaml:"waitingFor"`
	User            string               `yaml:"user" yaml:"user"`
}

func (cc *ContainerConfig) check(c *ComposeConfig) error {
	if cc == nil {
		return nil
	}
	for _, vm := range cc.VolumeMounts {
		if err := vm.check(c); err != nil {
			return err
		}
	}
	return cc.WaitingFor.check()
}

type VolumeMountConfig struct {
	Name      string `json:"name" yaml:"name"`
	MountPath string `json:"mountPath" yaml:"mountPath"`
}

func (vm *VolumeMountConfig) check(c *ComposeConfig) error {
	if vm == nil {
		return nil
	}
	volumeMap := make(map[string]string)
	for _, v := range c.Volumes {
		volumeMap[v.Name] = v.Name
	}
	if _, ok := volumeMap[vm.Name]; !ok {
		return errors.Errorf("%s is not found in volumes", vm.Name)
	}
	return nil
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
