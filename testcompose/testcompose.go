package testcompose

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"os"
	"path/filepath"
	"podcompose/common"
	"podcompose/compose"
	"podcompose/docker"
)

type TestCompose struct {
	agent          *compose.Agent
	compose        *compose.Compose
	workspace      string
	agentContainer docker.Container
}

func NewTestComposeWithSessionId(workspace string, sessionId string) (*TestCompose, error) {
	configByte, err := os.ReadFile(filepath.Join(workspace, common.ConfigFileName))
	if err != nil {
		return nil, err
	}
	c, err := compose.NewCompose(configByte, sessionId, workspace)
	if err != nil {
		return nil, err
	}
	return &TestCompose{compose: c, agent: compose.NewAgent(c), workspace: workspace}, nil
}

func NewTestCompose(workspace string) (*TestCompose, error) {
	return NewTestComposeWithSessionId(workspace, "")
}
func (t *TestCompose) GetSessionId() string {
	return t.compose.GetConfig().SessionId
}
func (t *TestCompose) verify(ctx context.Context) error {
	containers, err := t.compose.GetDockerProvider().FindContainers(ctx, t.compose.GetConfig().SessionId)
	if err != nil {
		return err
	}
	if len(containers) != 0 {
		return errors.Errorf("session name:%s is exist in system, please change name and try again", t.compose.GetConfig().SessionId)
	}
	return nil
}

func (t *TestCompose) Start(ctx context.Context) error {
	if err := t.verify(ctx); err != nil {
		return err
	}
	// first prepare Network and Volumes
	err := t.compose.PrepareNetwork(ctx)
	if err != nil {
		return err
	}
	agentContainer, err := t.agent.StartAgentForServer(ctx)
	if err != nil {
		return err
	}
	t.agentContainer = agentContainer
	return nil
}

func (t *TestCompose) GetPort(ctx context.Context) (string, error) {
	ports, err := t.agentContainer.Ports(ctx)
	if err != nil {
		return "", err
	}
	for _, portBindings := range ports {
		if len(portBindings) == 0 {
			continue
		}
		return portBindings[0].HostPort, nil
	}
	return "", errors.New("can not found managed port")
}

type AgentLogConsumer struct {
}

func (a AgentLogConsumer) Accept(log docker.Log) {
	fmt.Print(string(log.Content))
}

func (t *TestCompose) ShowAgentLog(ctx context.Context) error {
	t.agentContainer.FollowOutput(&AgentLogConsumer{})
	err := t.agentContainer.StartLogProducer(ctx)
	if err != nil {
		return err
	}
	select {}
}
