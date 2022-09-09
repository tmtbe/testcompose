package main

import (
	"context"
	"go.uber.org/zap"
	"podcompose/compose"
	"podcompose/docker"
)

type StopCmd struct {
	names  []string
	agents []*compose.Agent
	plist  *Plist
}
type SampleCompose struct {
	dockerProvider *docker.DockerProvider
	sessionId      string
}

func (s SampleCompose) GetContextPathForMount() string {
	panic("not need")
}

func (s SampleCompose) GetDockerProvider() *docker.DockerProvider {
	return s.dockerProvider
}

func (s SampleCompose) GetSessionId() string {
	return s.sessionId
}

func (s SampleCompose) GetConfig() *compose.ComposeConfig {
	panic("not need")
}
func NewSampleCompose(sessionId string, dockerProvider *docker.DockerProvider) (*SampleCompose, error) {
	return &SampleCompose{
		dockerProvider: dockerProvider,
		sessionId:      sessionId,
	}, nil
}
func NewStopCmd(names []string) (*StopCmd, error) {
	dockerProvider, err := docker.NewDockerProvider()
	if err != nil {
		return nil, err
	}
	agents := make([]*compose.Agent, len(names))
	for i, name := range names {
		sampleCompose, err := NewSampleCompose(name, dockerProvider)
		if err != nil {
			return nil, err
		}
		agents[i] = compose.NewAgent(sampleCompose)
	}
	plist := NewPlist(dockerProvider)
	if err != nil {
		return nil, err
	}
	return &StopCmd{
		names:  names,
		agents: agents,
		plist:  plist,
	}, nil
}

func (s *StopCmd) Stop() error {
	ctx := context.Background()
	ps, err := s.plist.ps(ctx)
	if err != nil {
		return err
	}
	for _, agent := range s.agents {
		if _, ok := ps[agent.GetSessionId()]; !ok {
			zap.L().Sugar().Warnf("%s is not exist", agent.GetSessionId())
			continue
		}
		err := agent.StartAgentForClean(ctx)
		if err != nil {
			zap.L().Sugar().Error("stop %s failed", agent.GetSessionId())
			return err
		}
		zap.L().Sugar().Infof("stop %s success", agent.GetSessionId())
	}
	return nil
}
