package compose

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"podcompose/common"
	"podcompose/docker"
	"podcompose/event"
	"time"
)

type Observe struct {
	ids   map[string]string
	cache map[string]event.ContainerEventData
}

func (o *Observe) Start(provider *docker.DockerProvider) {
	o.ids = make(map[string]string)
	o.cache = make(map[string]event.ContainerEventData)
	go func() {
		zap.L().Debug("Start observe")
		ctx := context.Background()
		for true {
			for index, id := range o.ids {
				inspect, err := provider.ContainerInspect(ctx, id)
				if err != nil {
					delete(o.ids, index)
					continue
				}
				eventData := event.ContainerEventData{
					PodName:       inspect.Config.Labels[common.LabelPodName],
					ContainerName: inspect.Config.Labels[common.LabelContainerName],
					Id:            id,
					Name:          inspect.Name,
					Image:         inspect.Image,
					Type:          event.ContainerEventStateType,
					State:         inspect.State,
				}
				if !o.isRepeat(eventData) {
					event.Publish(&eventData)
				}
				if inspect.State.Running == false && inspect.State.ExitCode != 0 {
					event.Publish(&event.ErrorData{
						Reason:  "Error exit code",
						Message: fmt.Sprintf("Pod [%s] Container [%s] is dead and exit code is not 0", inspect.Config.Labels[common.LabelPodName], inspect.Config.Labels[common.LabelContainerName]),
					})
				}
			}
			time.Sleep(time.Second)
		}
	}()
}

func (o *Observe) observeContainerId(id string) {
	o.ids[id] = id
}

func (o *Observe) isRepeat(data event.ContainerEventData) bool {
	old, ok := o.cache[data.Name]
	if ok {
		if old.Id != data.Id {
			o.cache[data.Name] = data
			return false
		}
		if old.State.Status != data.State.Status {
			o.cache[data.Name] = data
			return false
		}
	} else {
		o.cache[data.Name] = data
	}
	return true
}
