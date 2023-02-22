package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"os"
	"podcompose/common"
	"podcompose/event"
	"strings"
)

func main() {
	InitLogger()
	zap.L().Info("Start agent")
	sessionId := os.Getenv(common.LabelSessionID)
	hostContextPath := os.Getenv(common.EnvHostContextPath)
	runner, err := NewStarter(common.AgentContextPath, sessionId, hostContextPath)
	volume, err := NewVolume(common.AgentContextPath, sessionId)
	cleaner, err := NewCleaner(sessionId)
	handleError(err)
	rootCmd := &cobra.Command{
		Use: "agent",
	}
	cleanCmd := &cobra.Command{
		Use: "clean",
		Run: func(cmd *cobra.Command, args []string) {
			cleaner.clear()
		},
	}
	prepareVolumeCmd := &cobra.Command{
		Use: "prepareVolume",
		Run: func(cmd *cobra.Command, args []string) {
			handleError(err)
			err = volume.copyDataToVolume(volume.GetConfig().Volumes)
			handleError(err)
		},
	}
	prepareVolumeGroupCmd := &cobra.Command{
		Use: "prepareVolumeGroup",
		Run: func(cmd *cobra.Command, args []string) {
			selectGroupIndex, err := cmd.Flags().GetInt("selectGroupIndex")
			handleError(err)
			err = volume.copyDataToVolumeGroup(selectGroupIndex)
			handleError(err)
		},
	}
	prepareVolumeGroupCmd.Flags().IntP("selectGroupIndex", "s", 0, "select volume group")
	startCmd := &cobra.Command{
		Use: "start",
		Run: func(cmd *cobra.Command, args []string) {
			err = event.StartEventBusServer()
			if err != nil {
				handleError(err)
			}
			autoStart, err := cmd.Flags().GetBool("autoStart")
			if err != nil {
				handleError(err)
			}
			if autoStart {
				zap.L().Info("Auto start mode is enable, start compose now")
				go func() {
					if err = runner.start(); err != nil {
						handleError(err)
					}
				}()
			}
			if err = runner.startWebServer(); err != nil {
				handleError(err)
			}
		},
	}
	startCmd.Flags().Bool("autoStart", true, "auto start compose")
	prepareIngressVolumeCmd := &cobra.Command{
		Use: "prepareIngressVolume",
		Run: func(cmd *cobra.Command, args []string) {
			servicePorts, err := cmd.Flags().GetStringArray("ports")
			handleError(err)
			servicePortMap := make(map[string]string)
			for _, servicePort := range servicePorts {
				servicePortSplit := strings.Split(servicePort, "=")
				servicePortMap[servicePortSplit[0]] = servicePortSplit[1]
			}
			err = runner.prepareIngressVolume(servicePortMap)
			handleError(err)
		},
	}
	prepareIngressVolumeCmd.Flags().StringArrayP("ports", "p", []string{}, "service port mapping")
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(prepareVolumeCmd)
	rootCmd.AddCommand(prepareVolumeGroupCmd)
	rootCmd.AddCommand(prepareIngressVolumeCmd)
	err = rootCmd.Execute()
	handleError(err)
}

func handleError(err error) {
	if err != nil {
		zap.L().Sugar().Error(err)
		os.Exit(1)
	}
}
