package compose

import (
	"context"
	"fmt"
	"podcompose/docker"
	"strings"
)

type AgentLog struct {
	Name string
}

func (a *AgentLog) Accept(log docker.Log) {
	fmt.Printf("%s | %s", strings.TrimLeft(a.Name, "/"), string(log.Content))
}
func collectLogs(name *string, container docker.Container) {
	ctx := context.Background()
	if name == nil {
		cname, err := container.Name(ctx)
		if err != nil {
			return
		}
		name = &cname
	}
	container.FollowOutput(&AgentLog{
		Name: *name,
	})
	_ = container.StartLogProducer(ctx)
}
