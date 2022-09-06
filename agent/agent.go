package main

import (
	"github.com/spf13/cobra"
	"os"
	"podcompose/common"
	"strings"
)

func main() {
	sessionId := os.Getenv(common.AgentSessionID)
	hostContextPath := os.Getenv(common.HostContextPath)
	runner, err := NewStarter(common.AgentContextPath, sessionId, hostContextPath)
	volume, err := NewVolume(common.AgentContextPath, sessionId)
	cleaner, err := NewCleaner(sessionId)
	if err != nil {
		panic(err)
	}
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
			if err != nil {
				panic(err)
			}
			selectMap := make(map[string]string)
			for _, selectGroup := range selectArr {
				selectGroupSplit := strings.Split(selectGroup, "=")
				selectMap[selectGroupSplit[0]] = selectGroupSplit[1]
			}
			err = volume.copyDataToVolumes(selectMap)
			if err != nil {
				panic(err)
			}
		},
	}
	prepareVolumeDataCmd.Flags().StringArrayP("select", "s", []string{}, "select volume and switch data")
	startCmd := &cobra.Command{
		Use: "start",
		Run: func(cmd *cobra.Command, args []string) {
			go func() {
				if err = runner.start(); err != nil {
					panic(err)
				}
			}()
			if err = runner.startWebServer(); err != nil {
				panic(err)
			}
		},
	}
	reStartCmd := &cobra.Command{
		Use: "switch",
		Run: func(cmd *cobra.Command, args []string) {
			selectArr, err := cmd.Flags().GetStringArray("select")
			if err != nil {
				panic(err)
			}
			selectMap := make(map[string]string)
			for _, selectGroup := range selectArr {
				selectGroupSplit := strings.Split(selectGroup, "=")
				selectMap[selectGroupSplit[0]] = selectGroupSplit[1]
			}
			err = runner.switchData(selectMap)
			if err != nil {
				panic(err)
			}
		},
	}
	reStartCmd.Flags().StringArrayP("select", "s", []string{}, "select volume and switch data")
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(reStartCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(prepareVolumeDataCmd)
	err = rootCmd.Execute()
	if err != nil {
		panic(err)
	}
}
