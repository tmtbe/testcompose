package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"os"
	"path/filepath"
	"podcompose/common"
	"podcompose/compose"
	"time"
)

type Starter struct {
	*compose.Compose
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
		Compose:         c,
		hostContextPath: hostContextPath,
	}, nil
}

func (s *Starter) start() error {
	ctx := context.Background()
	selectData := make(map[string]string)
	for _, v := range s.Compose.GetConfig().Volumes {
		selectData[v.Name] = "normal"
	}
	err := s.Compose.CreateVolumes(ctx)
	if err != nil {
		return err
	}
	err = s.Compose.StartAgentForSetVolume(ctx, selectData)
	if err != nil {
		return err
	}
	return s.Compose.StartPods(ctx)
}

func (s *Starter) restart(podNames []string) error {
	ctx := context.Background()
	return s.Compose.RestartPods(ctx, podNames, func() error {
		return nil
	})
}

func (s *Starter) startWebServer() error {
	quit := make(chan bool, 1)
	router := gin.Default()
	router.GET(common.AgentHealthEndPoint, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "ok",
		})
	})
	router.GET(common.AgentShutdownEndPoint, func(c *gin.Context) {
		ctx := context.Background()
		_ = s.StartAgentForClean(ctx)
		quit <- true
		c.JSON(http.StatusOK, gin.H{
			"message": "shutdown",
		})
	})
	router.POST(common.AgentSwitchDataEndPoint, func(c *gin.Context) {
		ctx := context.Background()
		type SwitchDataBody map[string]string
		var switchDataBody SwitchDataBody
		err := c.BindJSON(&switchDataBody)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": err.Error(),
			})
			return
		}
		err = s.StartAgentForSwitchData(ctx, switchDataBody)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "switch data ok",
		})
	})
	router.POST(common.AgentRestartEndPoint, func(c *gin.Context) {
		ctx := context.Background()
		type RestartBody []string
		var restartBody RestartBody
		err := c.BindJSON(&restartBody)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": err.Error(),
			})
			return
		}
		err = s.StartAgentForRestart(ctx, restartBody)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "restart ok",
		})
	})
	srv := &http.Server{
		Addr:    ":" + common.AgentPort,
		Handler: router,
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
	for volumeName, _ := range selectData {
		volumeNames = append(volumeNames, volumeName)
	}
	pods := s.Compose.FindPodsWhoUsedVolumes(volumeNames)
	podNames := make([]string, len(pods))
	for k, v := range pods {
		podNames[k] = v.Name
	}
	return s.Compose.RestartPods(ctx, podNames, func() error {
		err := s.Compose.RecreateVolumes(ctx, volumeNames)
		if err != nil {
			return err
		}
		return s.Compose.StartAgentForSetVolume(ctx, selectData)
	})
}
