package main

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"path/filepath"
	"podcompose/common"
)

func InitLogger() {
	encoder := getEncoder()
	debug := os.Getenv(common.TpcDebug)
	tpcName := os.Getenv(common.TpcName)
	fileWriteSyncer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   filepath.Join(common.AgentLogPath, tpcName+".log"), //日志文件存放目录
		MaxSize:    10,                                                 //文件大小限制,单位MB
		MaxBackups: 5,                                                  //最大保留日志文件数量
		MaxAge:     30,                                                 //日志文件保留天数
		Compress:   false,                                              //是否压缩处理
	})
	var core zapcore.Core
	if debug != "" {
		core = zapcore.NewCore(encoder, zapcore.NewMultiWriteSyncer(fileWriteSyncer, zapcore.AddSync(os.Stdout)), zap.DebugLevel)
	} else {
		core = zapcore.NewCore(encoder, zapcore.NewMultiWriteSyncer(fileWriteSyncer, zapcore.AddSync(os.Stdout)), zap.InfoLevel)
	}
	logger := zap.New(core)
	zap.ReplaceGlobals(logger)
}
func getEncoder() zapcore.Encoder {
	return zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
}
