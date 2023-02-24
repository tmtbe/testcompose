package compose

import (
	"github.com/docker/docker/api/types"
	"github.com/pkg/errors"
	"github.com/sony/sonyflake"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
	"path/filepath"
	"podcompose/common"
	"podcompose/docker"
	"podcompose/event"
	"strconv"
	"sync"
)

type Compose struct {
	podCompose      *PodCompose
	config          *ComposeConfig
	dockerProvider  *docker.DockerProvider
	volume          *VolumeGroups
	contextPath     string
	hostContextPath string
	ready           bool
	triggerLock     sync.Mutex
}

func NewCompose(configBytes []byte, sessionId string, contextPath string, hostContextPath string) (*Compose, error) {
	contextPath, err := filepath.Abs(contextPath)
	if err != nil {
		return nil, err
	}
	var config ComposeConfig
	err = yaml.Unmarshal(configBytes, &config)
	if err != nil {
		return nil, err
	}
	config.SessionId = sessionId
	if config.SessionId == "" {
		config.SessionId = genSessionId()
	}
	err = config.check(contextPath)
	if err != nil {
		return nil, err
	}
	provider, err := docker.NewDockerProvider()
	if err != nil {
		return nil, err
	}
	compose, err := NewPodCompose(sessionId, hostContextPath, config.Pods, config.GetNetworkName(), provider)
	if err != nil {
		return nil, err
	}
	return &Compose{
		podCompose:      compose,
		config:          &config,
		dockerProvider:  provider,
		volume:          NewVolumeGroups(config.VolumeGroups, provider),
		contextPath:     contextPath,
		hostContextPath: hostContextPath,
	}, nil
}

func genSessionId() string {
	var st sonyflake.Settings
	id, _ := sonyflake.NewSonyflake(st).NextID()
	return strconv.FormatInt(int64(id), 16)
}

func (c *Compose) StartPods(ctx context.Context) error {
	eventData := event.ComposeEventData{
		Type:    event.ComposeEventBeforeStartType,
		Trigger: c.SystemAutoTrigger,
	}
	event.Publish(ctx, &eventData)
	zap.L().Info("Compose start running")
	err := c.podCompose.start(ctx)
	if err != nil {
		eventData = event.ComposeEventData{
			Type:    event.ComposeEventStartFailType,
			Trigger: c.SystemAutoTrigger,
		}
	} else {
		c.ready = true
		eventData = event.ComposeEventData{
			Type:    event.ComposeEventStartSuccessType,
			Trigger: c.SystemAutoTrigger,
		}
		zap.L().Info("Compose is ready, all pods is started")
	}
	event.Publish(ctx, &eventData)
	return err
}

func (c *Compose) CreateVolumesWithGroup(ctx context.Context, defaultGroup *VolumeGroupConfig) error {
	return c.volume.createVolumesWithGroup(ctx, c.GetConfig().SessionId, defaultGroup)
}
func (c *Compose) CreateVolumes(ctx context.Context, volumes []*VolumeConfig) error {
	return c.volume.createVolumes(ctx, c.GetConfig().SessionId, volumes)
}
func (c *Compose) CreateSystemLogVolume(ctx context.Context) (types.Volume, error) {
	return c.volume.createVolume(ctx, c.GetConfig().SessionId, common.SystemLogVolumeName)
}
func (c *Compose) RecreateVolumesWithGroup(ctx context.Context, volumeGroup *VolumeGroupConfig) error {
	return c.volume.recreateVolumesWithGroup(ctx, volumeGroup, c.GetConfig().SessionId)
}

func (c *Compose) FindPodsWhoUsedVolumes(volumeNames []string) []*PodConfig {
	return c.podCompose.findPodsWhoUsedVolumes(volumeNames)
}

func (c *Compose) RestartPods(ctx context.Context, podNames []string, beforeStart func() error) error {
	if !c.ready {
		return errors.New("compose is not ready, can not restart")
	}
	for _, podName := range podNames {
		if _, ok := c.podCompose.pods[podName]; !ok {
			return errors.Errorf("pod name:%s is not exist", podName)
		}
	}
	eventData := event.ComposeEventData{
		Type:    event.ComposeEventBeforeRestartType,
		Trigger: c.SystemAutoTrigger,
	}
	event.Publish(ctx, &eventData)
	zap.L().Info("Compose restart pods")
	c.ready = false
	err := c.podCompose.RestartPods(ctx, podNames, beforeStart)
	if err == nil {
		c.ready = true
		eventData = event.ComposeEventData{
			Type:    event.ComposeEventRestartSuccessType,
			Trigger: c.SystemAutoTrigger,
		}
		zap.L().Info("Compose restart pods success")
	} else {
		eventData = event.ComposeEventData{
			Type:    event.ComposeEventRestartFailType,
			Trigger: c.SystemAutoTrigger,
		}
	}
	event.Publish(ctx, &eventData)
	return err
}

func (c *Compose) IsReady() bool {
	return c.ready
}

func (c *Compose) GetContextPathForMount() string {
	if c.hostContextPath != "" {
		return c.hostContextPath
	} else {
		return c.contextPath
	}
}

func (c *Compose) GetConfig() *ComposeConfig {
	return c.config
}

func (c *Compose) GetSessionId() string {
	return c.config.SessionId
}

func (c *Compose) GetDockerProvider() *docker.DockerProvider {
	return c.dockerProvider
}

// PrepareNetwork network and volumes should be init before agent start
func (c *Compose) PrepareNetwork(ctx context.Context) error {
	if c.config.Network == "" {
		_, err := c.dockerProvider.CreateNetwork(ctx, docker.NetworkRequest{
			Driver:         docker.Bridge,
			CheckDuplicate: true,
			Name:           c.config.GetNetworkName(),
		}, c.config.SessionId)
		if err != nil {
			return err
		}
	} else {
		_, err := c.dockerProvider.GetNetwork(ctx, docker.NetworkRequest{
			Name: c.config.Network,
		})
		if err != nil {
			return errors.Errorf("network: %s is not exist", c.config.Network)
		}
	}
	return nil
}

func (c *Compose) SystemAutoTrigger(ctx context.Context, eventName string) error {
	taskGroups := c.config.TaskGroups.GetTaskGroupFromEvent(eventName)
	for _, taskGroup := range taskGroups {
		taskGroup := taskGroup
		go func() {
			err := c.podCompose.StartTrigger("system_trigger_"+taskGroup.Name, taskGroup.Tasks, ctx)
			eventData := event.ComposeEventData{
				Type:    event.ComposeEventTriggerFinishTask + ":" + taskGroup.Name,
				Trigger: c.SystemAutoTrigger,
			}
			event.Publish(ctx, &eventData)
			if err != nil {
				zap.L().Sugar().Error("SystemAutoTrigger Error: ", err)
				event.Publish(ctx, &event.ErrorData{
					Reason:  "SystemAutoTrigger Error",
					Message: err.Error(),
				})
			}
		}()
	}
	return nil
}

func (c *Compose) StartUserTrigger(ctx context.Context, name string) error {
	if !c.ready {
		return errors.Errorf("compose is not ready, can not trigger task")
	}
	taskGroup := c.config.TaskGroups.GetTaskGroupFromName(name)
	if taskGroup == nil {
		return nil
	}
	c.triggerLock.Lock()
	defer c.triggerLock.Unlock()
	triggerName := "user_trigger_" + name
	err := c.podCompose.StartTrigger(triggerName, taskGroup.Tasks, ctx)
	eventData := event.ComposeEventData{
		Type:    event.ComposeEventTriggerFinishTask + ":" + name,
		Trigger: c.SystemAutoTrigger,
	}
	event.Publish(ctx, &eventData)
	return err
}

func (c *Compose) StopPods(ctx context.Context) {
	c.ready = false
	eventData := event.ComposeEventData{
		Type:    event.ComposeEventBeforeStopType,
		Trigger: c.SystemAutoTrigger,
	}
	event.Publish(ctx, &eventData)
	cs, err := c.dockerProvider.FindAllContainersWithSessionId(ctx, c.GetSessionId())
	if err != nil {
		return
	}
	for _, container := range cs {
		if container.Labels[docker.AgentType] == docker.AgentTypeServer {
			continue
		}
		_ = c.dockerProvider.RemoveContainer(ctx, container.ID)
	}
	vs, err := c.dockerProvider.FindAllVolumesWithSessionId(ctx, c.GetSessionId())
	if err != nil {
		return
	}
	for _, volume := range vs {
		_ = c.dockerProvider.RemoveVolume(ctx, volume.Name, c.GetSessionId(), true)
	}
	eventData = event.ComposeEventData{
		Type:    event.ComposeEventAfterStopType,
		Trigger: c.SystemAutoTrigger,
	}
	event.Publish(ctx, &eventData)
}
