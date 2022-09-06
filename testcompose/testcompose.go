package testcompose

import (
	"context"
	"go.uber.org/zap"
	"os"
	"path/filepath"
	"podcompose/common"
	"podcompose/compose"
	"podcompose/docker"
)

type TestCompose struct {
	compose        *compose.Compose
	workspace      string
	agentContainer docker.Container
}

func NewTestCompose(workspace string, sessionId string) (*TestCompose, error) {
	configByte, err := os.ReadFile(filepath.Join(workspace, common.ConfigFileName))
	if err != nil {
		return nil, err
	}
	c, err := compose.NewCompose(configByte, sessionId, workspace)
	if err != nil {
		return nil, err
	}
	return &TestCompose{compose: c, workspace: workspace}, nil
}

func (t *TestCompose) Start(ctx context.Context) error {
	// first prepare Network and Volumes
	err := t.compose.PrepareNetwork(ctx)
	if err != nil {
		return err
	}
	agentContainer, err := t.compose.StartAgentForServer(ctx)
	if err != nil {
		return err
	}
	t.agentContainer = agentContainer
	return nil
}

type AgentLogConsumer struct {
}

func (a AgentLogConsumer) Accept(log docker.Log) {
	zap.L().Sugar().Info(string(log.Content))
}

func (t *TestCompose) ShowAgentLog(ctx context.Context) error {
	t.agentContainer.FollowOutput(&AgentLogConsumer{})
	err := t.agentContainer.StartLogProducer(ctx)
	if err != nil {
		return err
	}
	select {}
}
