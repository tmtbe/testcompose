package event

import (
	"encoding/json"
	"fmt"
	"github.com/docker/docker/api/types"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol/pub"
	_ "go.nanomsg.org/mangos/v3/transport/all"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"podcompose/common"
	"time"
)

var Bus *EventBus

type EventBus struct {
	sock mangos.Socket
}

type EventMsg struct {
	Topic              string
	ComposeEventData   *ComposeEventData
	PodEventData       *PodEventData
	ContainerEventData *ContainerEventData
	TaskGroupEventData *TaskGroupEventData
	TaskEventData      *TaskEventData
	ErrorData          *ErrorData
}

func (e *EventMsg) ToJson() string {
	jsonbody, _ := json.Marshal(e)
	return string(jsonbody)
}
func (e *EventBus) Publish(event Event) error {
	eventMsg := event.ToMessage()
	return e.sock.Send([]byte(eventMsg.ToJson()))
}

func StartEventBusServer() error {
	var sock mangos.Socket
	var err error
	if sock, err = pub.NewSocket(); err != nil {
		return err
	}
	if err = sock.Listen(fmt.Sprintf("tcp://0.0.0.0:%s", common.ServerAgentEventBusPort)); err != nil {
		return err
	}
	Bus = &EventBus{sock: sock}
	return nil
}

const Compose string = "compose"
const ComposeEventBeforeStartType = "compose_event_before_start"
const ComposeEventStartSuccessType = "compose_event_start_success"
const ComposeEventStartFailType = "compose_event_start_fail"
const ComposeEventBeforeRestartType = "compose_event_before_restart"
const ComposeEventRestartSuccessType = "compose_event_restart_success"
const ComposeEventRestartFailType = "compose_event_restart_fail"
const ComposeEventBeforeStopType = "compose_event_before_stop"
const ComposeEventAfterStopType = "compose_event_after_stop"
const ComposeEventTaskGroupSuccess = "compose_event_task_group_success"
const ComposeEventAutoTaskGroupAllFinish = "compose_event_auto_task_group_all_finish"

const Error string = "error"

const Pod string = "pod"
const PodEventStartType = "start"
const PodEventReadyType = "ready"

const TaskGroup = "taskGroup"
const TaskGroupEventTaskGroupStart = "task_group_event_start"
const TaskGroupEventTaskGroupSuccess = "task_group_event_success"

const Task = "task"
const TaskEventTaskStart = "task_event_start"
const TaskEventTaskSuccess = "task_event_success"

const Container string = "container"
const ContainerEventPullStartType = "container_event_pull_start"
const ContainerEventPullSuccessType = "container_event_pull_success"
const ContainerEventPullFailType = "container_event_pull_fail"
const ContainerEventCreatedType = "container_event_container_created"
const ContainerEventStartType = "container_event_container_start"
const ContainerEventReadyType = "container_event_container_ready"
const ContainerEventRemoveType = "container_event_container_remove"
const ContainerEventStateType = "container_event_container_state"

func Publish(event Event) {
	go func() {
		err := event.Do()
		if err != nil {
			zap.L().Sugar().Errorf("event %s do error %s", event.ToMessage().ToJson(), err)
		}
	}()
	event.SetEventTime(time.Now())
	if Bus != nil {
		err := Bus.Publish(event)
		if err != nil {
			zap.L().Sugar().Errorf("send event error")
		}
	}
	zap.L().Sugar().Debugf("event[%s]: %s", event.Topic(), event.ToMessage().ToJson())
}

type Event interface {
	SetEventTime(eventTime time.Time)
	ToMessage() *EventMsg
	Topic() string
	Do() error
}

type PodEventData struct {
	Name      string
	Type      string
	EventTime time.Time
	PodName   string
}

func (p *PodEventData) SetEventTime(eventTime time.Time) {
	p.EventTime = eventTime
}

func (p *PodEventData) ToMessage() *EventMsg {
	return &EventMsg{
		Topic:        p.Topic(),
		PodEventData: p,
	}
}

func (p *PodEventData) Topic() string {
	return Pod
}

func (p *PodEventData) Do() error {
	return nil
}

type ContainerEventData struct {
	Name          string
	Image         string
	Id            string
	Type          string
	EventTime     time.Time
	State         *types.ContainerState
	ContainerName string
	PodName       string
}

func (c *ContainerEventData) SetEventTime(eventTime time.Time) {
	c.EventTime = eventTime
}

func (c *ContainerEventData) ToMessage() *EventMsg {
	return &EventMsg{
		Topic:              c.Topic(),
		ContainerEventData: c,
	}
}

func (c *ContainerEventData) Topic() string {
	return Container
}

func (c *ContainerEventData) Do() error {
	return nil
}

type ComposeEventData struct {
	Type      string
	EventTime time.Time
	Trigger   func(ctx context.Context, name string) error `json:"-"`
}

func (c *ComposeEventData) SetEventTime(eventTime time.Time) {
	c.EventTime = eventTime
}

func (c *ComposeEventData) ToMessage() *EventMsg {
	return &EventMsg{
		Topic:            c.Topic(),
		ComposeEventData: c,
	}
}

func (c *ComposeEventData) Topic() string {
	return Compose
}

func (c *ComposeEventData) Do() error {
	if c.Trigger == nil {
		return nil
	}
	return c.Trigger(context.Background(), c.Type)
}

type ErrorData struct {
	EventTime time.Time
	Reason    string
	Message   string
}

func (c *ErrorData) SetEventTime(eventTime time.Time) {
	c.EventTime = eventTime
}

func (c *ErrorData) ToMessage() *EventMsg {
	return &EventMsg{
		Topic:     c.Topic(),
		ErrorData: c,
	}
}

func (c *ErrorData) Topic() string {
	return Error
}

func (c *ErrorData) Do() error {
	return nil
}

type TaskGroupEventData struct {
	Type          string
	TaskGroupName string
	EventTime     time.Time
}

func (t *TaskGroupEventData) SetEventTime(eventTime time.Time) {
	t.EventTime = eventTime
}

func (t *TaskGroupEventData) ToMessage() *EventMsg {
	return &EventMsg{
		Topic:              t.Topic(),
		TaskGroupEventData: t,
	}
}

func (t *TaskGroupEventData) Topic() string {
	return TaskGroup
}

func (t *TaskGroupEventData) Do() error {
	return nil
}

type TaskEventData struct {
	Type          string
	TaskGroupName string
	TaskName      string
	EventTime     time.Time
}

func (t *TaskEventData) SetEventTime(eventTime time.Time) {
	t.EventTime = eventTime
}

func (t *TaskEventData) ToMessage() *EventMsg {
	return &EventMsg{
		Topic:         t.Topic(),
		TaskEventData: t,
	}
}

func (t *TaskEventData) Topic() string {
	return Task
}

func (t *TaskEventData) Do() error {
	return nil
}
