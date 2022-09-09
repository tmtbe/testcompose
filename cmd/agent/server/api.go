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
	agent   *compose.Agent
	compose *compose.Compose
	quit    chan bool
}

func NewApi(c *compose.Compose, quit chan bool) *Api {
	return &Api{
		compose: c,
		quit:    quit,
		agent:   compose.NewAgent(c),
	}
}

func (a *Api) GetRoute() *gin.Engine {
	router := gin.New()
	router.Use(ginzap.Ginzap(zap.L(), time.RFC3339, true))
	router.Use(ginzap.RecoveryWithZap(zap.L(), true))
	router.GET(common.AgentHealthEndPoint, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "ok",
		})
	})
	router.GET(common.AgentShutdownEndPoint, func(c *gin.Context) {
		ctx := context.Background()
		go func() {
			_ = a.agent.StartAgentForClean(ctx)
		}()
		a.quit <- true
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
		err = a.agent.StartAgentForSwitchData(ctx, switchDataBody)
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
		err = a.agent.StartAgentForRestart(ctx, restartBody)
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
	return router
}
