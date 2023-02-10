package main

import (
	"context"
	"encoding/json"
	"go.uber.org/zap"
	"io/ioutil"
	"podcompose/common"
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

type ConfigDump struct {
	SessionId    string
	ManagerPort  string
	EventBusPort string
}

func (s *StartCmd) Start(autoStart bool, configDumpFile string) error {
	testCompose, err := testcompose.NewTestComposeWithSessionId(s.contextPath, s.name)
	if err != nil {
		return err
	}
	s.testCompose = testCompose
	ctx := context.Background()
	if err := testCompose.Start(ctx, autoStart); err != nil {
		return err
	}
	agentPort, err := testCompose.GetPort(ctx, common.ServerAgentPort)
	if err != nil {
		return err
	}
	eventBusPort, err := testCompose.GetPort(ctx, common.ServerAgentEventBusPort)
	if err != nil {
		return err
	}
	zap.L().Sugar().Infof("StartCmd test compose success, name is: %s, managed port is: %s, event bus port is:%s", testCompose.GetSessionId(), agentPort, eventBusPort)
	if configDumpFile != "" {
		configDump := &ConfigDump{
			SessionId:    testCompose.GetSessionId(),
			ManagerPort:  agentPort,
			EventBusPort: eventBusPort,
		}
		bytes, _ := json.Marshal(configDump)
		err := ioutil.WriteFile(configDumpFile, bytes, 0766)
		if err != nil {
			return err
		}
	}
	return nil
}
