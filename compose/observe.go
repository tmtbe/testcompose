package compose

import (
	"context"
	"go.uber.org/zap"
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
				state, err := provider.State(ctx, id)
				if err != nil {
					delete(o.ids, index)
				}
				event.Publish(ctx, event.Container, &event.ContainerEventData{
					Id:    id,
					Type:  event.ContainerEventStateType,
					State: *state,
				})
			}
			time.Sleep(time.Second)
		}
	}()
}

func (o *Observe) observeContainerId(id string) {
	o.ids[id] = id
}
