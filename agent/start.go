package main

import (
	"context"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"podcompose/common"
	"podcompose/compose"
	"time"
)

type Starter struct {
	*compose.Compose
}

func NewStarter(workspace string, sessionId string) (*Starter, error) {
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
	return &Starter{
		Compose: c,
	}, nil
}

func (r *Starter) start() error {
	err := r.copyDataToVolumes()
	if err != nil {
		return err
	}
	ctx := context.Background()
	return r.Compose.StartPods(ctx)
}

func (r *Starter) startWebServer() error {
	quit := make(chan bool, 1)
	router := gin.Default()
	router.GET(common.AgentHealthEndPoint, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "ok",
		})
	})
	router.GET(common.AgentShutdownEndPoint, func(c *gin.Context) {
		ctx := context.Background()
		_ = r.Clean(ctx)
		quit <- true
		c.JSON(http.StatusOK, gin.H{
			"message": "shutdown",
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

func (r *Starter) copyDataToVolumes() error {
	for _, v := range r.Compose.GetConfig().Volumes {
		if v.HostPath != "" {
			sourcePath := filepath.Join(common.AgentContextPath, v.HostPath)
			targetPath := filepath.Join(common.AgentVolumePath, v.Name)
			err := exec.Command("cp", "-r", sourcePath, targetPath).Run()
			if err != nil {
				return err
			}
		}
	}
	return nil
}
