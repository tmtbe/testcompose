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
	ids map[string]string
}

func (o *Observe) Start(provider *docker.DockerProvider) {
	o.ids = make(map[string]string)
	go func() {
		zap.L().Debug("Start observe")
		ctx := context.Background()
		for true {
			for index, id := range o.ids {
				inspect, err := provider.ContainerInspect(ctx, id)
				if err != nil {
					delete(o.ids, index)
				}
				event.Publish(ctx, &event.ContainerEventData{
					TracingData: event.TracingData{
						PodName:       inspect.Config.Labels[common.LabelPodName],
						ContainerName: inspect.Config.Labels[common.LabelContainerName],
					},
					Id:    id,
					Name:  inspect.Name,
					Image: inspect.Image,
					Type:  event.ContainerEventStateType,
					State: *inspect.State,
				})
			}
			time.Sleep(time.Second)
		}
	}()
}

func (o *Observe) observeContainerId(id string) {
	o.ids[id] = id
}
