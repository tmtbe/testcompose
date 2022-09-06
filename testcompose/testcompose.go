package testcompose

import (
	"context"
	"go.uber.org/zap"
	"os"
	"path/filepath"
	"podcompose/common"
	"podcompose/compose"
	"podcompose/docker"
	"podcompose/docker/wait"
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
	err := t.compose.PrepareNetworkAndVolumes(ctx)
	if err != nil {
		return err
	}
	// then start agent image, will mount context all files and all volumes, and agent will init volumes data, start services
	agentMounts := make([]docker.ContainerMount, 0)
	agentMounts = append(agentMounts, docker.BindMount("/var/run/docker.sock", "/var/run/docker.sock"))
	agentMounts = append(agentMounts, docker.BindMount(t.workspace, common.AgentContextPath))
	for _, v := range t.compose.GetConfig().Volumes {
		agentMounts = append(agentMounts, docker.VolumeMount(v.Name+"_"+t.compose.GetConfig().SessionId, docker.ContainerMountTarget(common.AgentVolumePath+v.Name)))
	}
	agentContainer, err := t.compose.GetDockerProvider().RunContainer(ctx, docker.ContainerRequest{
		Image:        common.AgentImage,
		Name:         "agent_" + t.compose.GetConfig().SessionId,
		ExposedPorts: []string{common.AgentPort},
		Mounts:       agentMounts,
		WaitingFor: wait.ForHTTP(common.AgentHealthEndPoint).
			WithPort(common.AgentPort + "/tcp").
			WithMethod("GET"),
		Env: map[string]string{
			common.AgentSessionID: t.compose.GetConfig().SessionId,
		},
		Networks: []string{t.compose.GetDockerProvider().GetDefaultNetwork(), t.compose.GetConfig().GetNetworkName()},
		NetworkAliases: map[string][]string{
			t.compose.GetConfig().GetNetworkName(): {"agent"},
		},
		Cmd: []string{"start"},
	}, t.compose.GetConfig().SessionId)
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
