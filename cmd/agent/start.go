package main

import (
	"context"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
	"net/http"
	"os"
	"path/filepath"
	"podcompose/cmd/agent/ingress"
	"podcompose/cmd/agent/server"
	"podcompose/common"
	"podcompose/compose"
	"strconv"
	"strings"
	"time"
)

type Starter struct {
	compose         *compose.Compose
	agent           *compose.Agent
	hostContextPath string
	isStarted       bool
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
	c, err := compose.NewCompose(configByte, sessionId, workspace, hostContextPath)
	if err != nil {
		return nil, err
	}
	return &Starter{
		compose:         c,
		agent:           compose.NewAgent(c),
		hostContextPath: hostContextPath,
		isStarted:       false,
	}, nil
}

func (s *Starter) start() error {
	if s.isStarted {
		return errors.New("compose is started")
	}
	defer func() {
		s.isStarted = true
	}()
	ctx := context.Background()
	// create volume
	err := s.compose.CreateVolumes(ctx, s.compose.GetConfig().Volumes)
	if err != nil {
		return err
	}
	err = s.agent.StartAgentForSetVolume(ctx)
	if err != nil {
		return err
	}
	// create volume group default
	if len(s.compose.GetConfig().VolumeGroups) > 0 {
		defaultGroup := s.compose.GetConfig().VolumeGroups[0]
		err := s.compose.CreateVolumesWithGroup(ctx, defaultGroup)
		if err != nil {
			return err
		}
		err = s.agent.StartAgentForSetVolumeGroup(ctx, 0)
		if err != nil {
			return err
		}
	}
	return s.compose.StartPods(ctx)
}
func (s *Starter) stop() error {
	if s.compose.IsReady() {
		ctx := context.Background()
		s.compose.StopPods(ctx)
		s.isStarted = false
	} else {
		return errors.New("compose is not ready, can not use stop command")
	}
	return nil
}

func (s *Starter) startWebServer() error {
	quit := make(chan bool, 1)
	api := server.NewApi(s.compose, quit, s.start, s.stop)
	srv := &http.Server{
		Addr:    ":" + common.ServerAgentPort,
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

func (s *Starter) prepareIngressVolume(servicePortMap map[string]string) error {
	config := ingress.NewEnvoyConfig()
	for serviceName, portMapping := range servicePortMap {
		portMappingSplit := strings.SplitN(portMapping, ":", 2)
		sourcePort, err := strconv.Atoi(portMappingSplit[0])
		if err != nil {
			return err
		}
		targetPort, err := strconv.Atoi(portMappingSplit[1])
		if err != nil {
			return err
		}
		err = config.AddExposePort(serviceName, sourcePort, targetPort)
		if err != nil {
			return err
		}
	}
	marshal, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(common.AgentVolumePath, common.IngressVolumeName, "envoy.yaml"), marshal, 0766)
	if err != nil {
		return err
	}
	return nil
}
