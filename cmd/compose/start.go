package main

import (
	"context"
	"go.uber.org/zap"
	"podcompose/testcompose"
)

type StartCmd struct {
	contextPath string
	name        string
	testCompose *testcompose.TestCompose
}

func NewStartCmd(contextPath string, name string) *StartCmd {
	return &StartCmd{
		contextPath: contextPath,
		name:        name,
	}
}

func (s *StartCmd) Start() error {
	testCompose, err := testcompose.NewTestComposeWithSessionId(s.contextPath, s.name)
	if err != nil {
		return err
	}
	s.testCompose = testCompose
	ctx := context.Background()
	if err := testCompose.Start(ctx); err != nil {
		return err
	}
	port, err := testCompose.GetPort(ctx)
	if err != nil {
		return err
	}
	zap.L().Sugar().Infof("StartCmd test compose success, name is: %s, managed port is: %s", testCompose.GetSessionId(), port)
	err = s.testCompose.ShowAgentLog(ctx)
	if err != nil {
		return err
	}
	return nil
}
