package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"os"
	"podcompose/common"
	"strings"
)

func main() {
	InitLogger()
	sessionId := os.Getenv(common.AgentSessionID)
	hostContextPath := os.Getenv(common.HostContextPath)
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
	prepareVolumeDataCmd := &cobra.Command{
		Use: "prepareVolumeData",
		Run: func(cmd *cobra.Command, args []string) {
			selectArr, err := cmd.Flags().GetStringArray("select")
			handleError(err)
			selectMap := make(map[string]string)
			for _, selectGroup := range selectArr {
				selectGroupSplit := strings.Split(selectGroup, "=")
				selectMap[selectGroupSplit[0]] = selectGroupSplit[1]
			}
			err = volume.copyDataToVolumes(selectMap)
			handleError(err)
		},
	}
	prepareVolumeDataCmd.Flags().StringArrayP("select", "s", []string{}, "select volume and switch data")
	startCmd := &cobra.Command{
		Use: "start",
		Run: func(cmd *cobra.Command, args []string) {
			go func() {
				if err = runner.start(); err != nil {
					handleError(err)
				}
			}()
			if err = runner.startWebServer(); err != nil {
				handleError(err)
			}
		},
	}
	restartCmd := &cobra.Command{
		Use: "restart",
		Run: func(cmd *cobra.Command, args []string) {
			podNames, err := cmd.Flags().GetStringArray("select")
			if err = runner.restart(podNames); err != nil {
				handleError(err)
			}
		},
	}
	restartCmd.Flags().StringArrayP("select", "s", []string{}, "select pod to restart")
	switchCmd := &cobra.Command{
		Use: "switch",
		Run: func(cmd *cobra.Command, args []string) {
			selectArr, err := cmd.Flags().GetStringArray("select")
			handleError(err)
			selectMap := make(map[string]string)
			for _, selectGroup := range selectArr {
				selectGroupSplit := strings.Split(selectGroup, "=")
				selectMap[selectGroupSplit[0]] = selectGroupSplit[1]
			}
			err = runner.switchData(selectMap)
			handleError(err)
		},
	}
	switchCmd.Flags().StringArrayP("select", "s", []string{}, "select volume and switch data")
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(restartCmd)
	rootCmd.AddCommand(switchCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(prepareVolumeDataCmd)
	err = rootCmd.Execute()
	handleError(err)
}

func handleError(err error) {
	if err != nil {
		zap.L().Sugar().Error(err)
		os.Exit(1)
	}
}
