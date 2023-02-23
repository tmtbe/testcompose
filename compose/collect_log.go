package compose

import (
	"context"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"path/filepath"
	"podcompose/common"
	"podcompose/docker"
	"strings"
)

type AgentLog struct {
	Name   string
	Logger *zap.Logger
}

func (a *AgentLog) Accept(log docker.Log) {
	content := string(log.Content)
	content = strings.Trim(content, "\r\n")
	content = strings.Trim(content, "\n")
	a.Logger.Info(content)
}

func collectLogs(container docker.Container) {
	ctx := context.Background()
	cname, _ := container.Name(ctx)
	fileWriteSyncer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   filepath.Join(common.AgentLogPath, cname+".log"), //日志文件存放目录
		MaxSize:    10,                                               //文件大小限制,单位MB
		MaxBackups: 5,                                                //最大保留日志文件数量
		MaxAge:     30,                                               //日志文件保留天数
		Compress:   false,                                            //是否压缩处理
	})
	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoder()), fileWriteSyncer, zap.InfoLevel)
	logger := zap.New(core)
	container.FollowOutput(&AgentLog{
		Name:   cname,
		Logger: logger,
	})
	err := container.StartLogProducer(ctx)
	if err != nil {
		zap.L().Error(err.Error())
	}
}
func encoder() zapcore.EncoderConfig {
	return zapcore.EncoderConfig{
		// Keys can be anything except the empty string.
		TimeKey:        "T",
		LevelKey:       "L",
		NameKey:        "N",
		CallerKey:      "C",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "M",
		StacktraceKey:  "S",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    nil,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   nil,
	}
}
