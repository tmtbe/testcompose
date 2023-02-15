package compose

import (
	"context"
	"go.uber.org/zap"
	"podcompose/docker"
	"strings"
)

type AgentLog struct {
	Name string
}

func (a *AgentLog) Accept(log docker.Log) {
	zap.L().Sugar().Debugf("%s | %s", strings.TrimLeft(a.Name, "/"), string(log.Content))
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
