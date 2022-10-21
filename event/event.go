package event

import (
	"github.com/asaskevich/EventBus"
	"github.com/docker/docker/api/types"
	"golang.org/x/net/context"
	"time"
)

var Bus EventBus.Bus

func init() {
	Bus = EventBus.New()
}

const Pod string = "pod"
const PodEventStartType = "pod_event_start_type"
const PodEventReadyType = "pod_event_ready_type"

const Container string = "container"
const ContainerEventCreatedType = "container_event_created_type"
const ContainerEventStartType = "container_event_start_type"
const ContainerEventReadyType = "container_event_ready_type"
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

func PrepareTracingData(ctx context.Context, tracingData TracingData) context.Context {
	data, ok := ctx.Value("eventTracingData").(TracingData)
	if ok {
		data.MergeTracingData(tracingData)
		return context.WithValue(ctx, "eventTracingData", data)
	} else {
		return context.WithValue(ctx, "eventTracingData", tracingData)
	}
}

func Publish(ctx context.Context, topic string, event Event) {
	data, ok := ctx.Value("eventTracingData").(TracingData)
	if ok {
		event.MergeTracingData(data)
	}
	event.SetEventTime(time.Now())
	Bus.Publish(topic, event)
}

type Event interface {
	SetEventTime(eventTime time.Time)
	MergeTracingData(tracingData TracingData)
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

type ContainerEventData struct {
	TracingData
	Name      string
	Image     string
	Id        string
	Type      string
	EventTime time.Time
	State     types.ContainerState
}

func (c *ContainerEventData) SetEventTime(eventTime time.Time) {
	c.EventTime = eventTime
}
