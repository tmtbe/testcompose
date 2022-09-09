package main

import (
	"context"
	"go.uber.org/zap"
	"net/http"
	"os"
	"path/filepath"
	"podcompose/cmd/agent/server"
	"podcompose/common"
	"podcompose/compose"
	"time"
)

type Starter struct {
	compose         *compose.Compose
	agent           *compose.Agent
	hostContextPath string
}

func NewStarter(workspace string, sessionId string, hostContextPath string) (*Starter, error) {
	workspace, err := filepath.Abs(workspace)
	if err != nil {
		return nil, err
	}
	configByte, err := os.ReadFile(filepath.Join(workspace, common.ConfigFileName))
	if err != nil {
		return nil, err
	}
	c, err := compose.NewCompose(configByte, sessionId, workspace)
	if err != nil {
		return nil, err
	}
	c.SetHostContextPath(hostContextPath)
	return &Starter{
		compose:         c,
		agent:           compose.NewAgent(c),
		hostContextPath: hostContextPath,
	}, nil
}

func (s *Starter) start() error {
	ctx := context.Background()
	selectData := make(map[string]string)
	for _, v := range s.compose.GetConfig().Volumes {
		selectData[v.Name] = "normal"
	}
	err := s.compose.CreateVolumes(ctx)
	if err != nil {
		return err
	}
	err = s.agent.StartAgentForSetVolume(ctx, selectData)
	if err != nil {
		return err
	}
	return s.compose.StartPods(ctx)
}

func (s *Starter) restart(podNames []string) error {
	ctx := context.Background()
	return s.compose.RestartPods(ctx, podNames, func() error {
		return nil
	})
}

func (s *Starter) startWebServer() error {
	quit := make(chan bool, 1)
	api := server.NewApi(s.compose, quit)
	srv := &http.Server{
		Addr:    ":" + common.AgentPort,
		Handler: api.GetRoute(),
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			zap.L().Sugar().Fatalf("listen: %s\n", err)
		}
	}()
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		return err
	}
	// catching ctx.Done(). timeout of 5 seconds.
	select {
	case <-ctx.Done():
		zap.L().Sugar().Info("timeout of 5 seconds.")
	}
	zap.L().Sugar().Info("Server exiting")
	return nil
}

func (s *Starter) switchData(selectData map[string]string) error {
	ctx := context.Background()
	volumeNames := make([]string, 0)
	for volumeName := range selectData {
		volumeNames = append(volumeNames, volumeName)
	}
	pods := s.compose.FindPodsWhoUsedVolumes(volumeNames)
	podNames := make([]string, len(pods))
	for k, v := range pods {
		podNames[k] = v.Name
	}
	return s.compose.RestartPods(ctx, podNames, func() error {
		err := s.compose.RecreateVolumes(ctx, volumeNames)
		if err != nil {
			return err
		}
		return s.agent.StartAgentForSetVolume(ctx, selectData)
	})
}
