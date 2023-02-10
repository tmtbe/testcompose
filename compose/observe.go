package compose

import (
	"context"
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
				}
				eventData := &event.ContainerEventData{
					TracingData: event.TracingData{
						PodName:       inspect.Config.Labels[common.LabelPodName],
						ContainerName: inspect.Config.Labels[common.LabelContainerName],
					},
					Id:    id,
					Name:  inspect.Name,
					Image: inspect.Image,
					Type:  event.ContainerEventStateType,
					State: *inspect.State,
				}
				if !o.isRepeat(eventData) {
					event.Publish(ctx, eventData)
				}
			}
			time.Sleep(time.Second)
		}
	}()
}

func (o *Observe) observeContainerId(id string) {
	o.ids[id] = id
}

func (o *Observe) isRepeat(data *event.ContainerEventData) bool {
	old, ok := o.cache[data.Name]
	if ok {
		if old.Id != data.Id {
			o.cache[data.ContainerName] = *data
			return false
		}
		if old.State.Status != data.State.Status {
			o.cache[data.ContainerName] = *data
			return false
		}
	} else {
		o.cache[data.Name] = *data
	}
	return true
}
