package server

import (
	"context"
	ginzap "github.com/gin-contrib/zap"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"net/http"
	"podcompose/common"
	"podcompose/compose"
	"time"
)

type Api struct {
	agent    *compose.Agent
	compose  *compose.Compose
	startFuc func() error
	stopFuc  func() error
	quit     chan bool
}

func NewApi(c *compose.Compose, quit chan bool, startFuc func() error, stopFuc func() error) *Api {
	return &Api{
		compose:  c,
		quit:     quit,
		startFuc: startFuc,
		stopFuc:  stopFuc,
		agent:    compose.NewAgent(c),
	}
}

func (a *Api) GetRoute() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(ginzap.Ginzap(zap.L(), time.RFC3339, true))
	router.Use(ginzap.RecoveryWithZap(zap.L(), true))
	router.GET(common.EndPointAgentHealth, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "ok",
		})
	})
	router.POST(common.EndPointAgentStart, func(c *gin.Context) {
		err := a.startFuc()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
		} else {
			c.JSON(http.StatusOK, gin.H{
				"message": "ok",
			})
		}
	})
	router.POST(common.EndPointAgentStop, func(c *gin.Context) {
		err := a.stopFuc()
		if err == nil {
			c.JSON(http.StatusOK, gin.H{
				"message": "stop success",
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
		}
	})
	router.POST(common.EndPointAgentShutdown, func(c *gin.Context) {
		ctx := context.Background()
		go func() {
			_ = a.agent.StartAgentForClean(ctx)
		}()
		a.quit <- true
		c.JSON(http.StatusOK, gin.H{
			"message": "shutdown",
		})
	})
	router.POST(common.EndPointAgentSwitchData, func(c *gin.Context) {
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
		volumeNames := make([]string, 0)
		for volumeName := range switchDataBody {
			volumeNames = append(volumeNames, volumeName)
		}
		pods := a.compose.FindPodsWhoUsedVolumes(volumeNames)
		podNames := make([]string, len(pods))
		for k, v := range pods {
			podNames[k] = v.Name
		}
		err = a.compose.RestartPods(ctx, podNames, func() error {
			return a.agent.StartAgentForSwitchData(ctx, switchDataBody)
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "switch data ok",
		})
	})
	router.POST(common.EndPointAgentRestart, func(c *gin.Context) {
		ctx := context.Background()
		type RestartBody []string
		var restartBody RestartBody
		err := c.BindJSON(&restartBody)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}
		err = a.compose.RestartPods(ctx, restartBody, func() error {
			return nil
		})
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
	router.POST(common.EndPointAgentIngress, func(c *gin.Context) {
		ctx := context.Background()
		type IngressBody map[string]string
		var ingressBody IngressBody
		err := c.BindJSON(&ingressBody)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}
		_, err = a.agent.StartAgentForIngress(ctx, ingressBody)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message": "set ingress ok",
		})
	})
	router.GET(common.EndPointAgentInfo, func(c *gin.Context) {
		c.JSON(http.StatusOK, a.agent.GetInfo())
	})
	return router
}
