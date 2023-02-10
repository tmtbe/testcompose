package main

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
	"podcompose/common"
)

func InitLogger() {
	encoder := getEncoder()
	debug := os.Getenv(common.TpcDebug)
	var core zapcore.Core
	if debug != "" {
		core = zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), zap.DebugLevel)
	} else {
		core = zapcore.NewCore(encoder, zapcore.AddSync(os.Stdout), zap.InfoLevel)
	}
	logger := zap.New(core)
	zap.ReplaceGlobals(logger)
}
func getEncoder() zapcore.Encoder {
	return zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
}
