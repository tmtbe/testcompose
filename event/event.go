package event

import (
	"encoding/json"
	"github.com/asaskevich/EventBus"
	"github.com/docker/docker/api/types"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"podcompose/common"
	"time"
)

var Bus EventBus.Bus

func StartEventBusServer() error {
	server := EventBus.NewServer(":"+common.ServerAgentEventBusPort, common.ServerAgentEventBusPath, EventBus.New())
	Bus = server.EventBus()
	return server.Start()
}

const Pod string = "pod"
const PodEventStartType = "pod_event_start_type"
const PodEventReadyType = "pod_event_ready_type"

const Container string = "container"
const ContainerEventCreatedType = "container_event_created_type"
const ContainerEventStartType = "container_event_start_type"
const ContainerEventReadyType = "container_event_ready_type"
const ContainerEventRemoveType = "container_event_remove_type"
const ContainerEventStateType = "container_event_state_type"

type TracingData struct {
	PodName       string
	ContainerName string
}

func (t *TracingData) MergeTracingData(data TracingData) {
	if data.PodName != "" {
		t.PodName = data.PodName
	}
	if data.ContainerName != "" {
		t.ContainerName = data.ContainerName
	}
}

func Publish(ctx context.Context, event Event) {
	if Bus == nil {
		return
	}
	data, ok := ctx.Value("eventTracingData").(TracingData)
	if ok {
		event.MergeTracingData(data)
	}
	event.SetEventTime(time.Now())
	eventJson := event.ToJson()
	Bus.Publish(event.Topic(), eventJson)
	zap.L().Sugar().Info(eventJson)
}

type Event interface {
	SetEventTime(eventTime time.Time)
	MergeTracingData(tracingData TracingData)
	ToJson() string
	Topic() string
}

type PodEventData struct {
	TracingData
	Name      string
	Type      string
	EventTime time.Time
}

func (p *PodEventData) SetEventTime(eventTime time.Time) {
	p.EventTime = eventTime
}

func (p *PodEventData) ToJson() string {
	jsonByte, _ := json.Marshal(p)
	return string(jsonByte)
}

func (p *PodEventData) Topic() string {
	return Pod
}

type ContainerEventData struct {
	TracingData
	Name      string
	Image     string
	Id        string
	Type      string
	EventTime time.Time
	State     *types.ContainerState
}

func (c *ContainerEventData) SetEventTime(eventTime time.Time) {
	c.EventTime = eventTime
}

func (c *ContainerEventData) ToJson() string {
	jsonByte, _ := json.Marshal(c)
	return string(jsonByte)
}

func (c *ContainerEventData) Topic() string {
	return Container
}
