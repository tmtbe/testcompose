package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"os"
	"podcompose/common"
	_ "podcompose/event"
)

func main() {
	InitLogger()
	rootCmd := &cobra.Command{
		Use: "tpc",
	}
	startCmd := &cobra.Command{
		Use: "start",
		Run: func(cmd *cobra.Command, args []string) {
			debug, err := cmd.Flags().GetBool("debug")
			if err != nil {
				handleError(err)
			}
			if debug {
				_ = os.Setenv("TPC_DEBUG", "true")
				common.AgentAutoRemove = false
				InitLogger()
			}
			autoStart, err := cmd.Flags().GetBool("autoStart")
			if err != nil {
				handleError(err)
			}
			configDumpFile, err := cmd.Flags().GetString("configDumpFile")
			if err != nil {
				handleError(err)
			}
			contextPath, err := cmd.Flags().GetString("path")
			handleError(err)
			name, err := cmd.Flags().GetString("name")
			handleError(err)
			start := NewStartCmd(contextPath, name)
			handleError(start.Start(autoStart, configDumpFile))
		},
	}
	wdPath, _ := os.Getwd()
	startCmd.Flags().Bool("debug", false, "debug mode")
	startCmd.Flags().Bool("autoStart", true, "auto start compose")
	startCmd.Flags().String("configDumpFile", "", "dump config file")
	startCmd.Flags().StringP("path", "p", wdPath, "context path, normal is $PWD")
	startCmd.Flags().StringP("name", "n", "", "set the test compose name, normal is uuid")
	shutdownCmd := &cobra.Command{
		Use: "shutdown",
		Run: func(cmd *cobra.Command, args []string) {
			stop, err := NewShutdownCmd(args)
			handleError(err)
			handleError(stop.Shutdown())
		},
	}
	psCmd := &cobra.Command{
		Use: "ps",
		Run: func(cmd *cobra.Command, args []string) {
			plist, err := NewPlistCmd()
			handleError(err)
			handleError(plist.Ps())
		},
	}
	cleanCmd := &cobra.Command{
		Use: "clean",
		Run: func(cmd *cobra.Command, args []string) {
			all, err := cmd.Flags().GetBool("all")
			handleError(err)
			cleanCmd, err := NewCleanCmd()
			handleError(err)
			handleError(cleanCmd.clean(all))
		},
	}
	cleanCmd.Flags().BoolP("all", "a", false, "all tpc")
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(shutdownCmd)
	rootCmd.AddCommand(psCmd)
	rootCmd.AddCommand(cleanCmd)
	err := rootCmd.Execute()
	handleError(err)
}
func handleError(err error) {
	if err != nil {
		zap.L().Sugar().Error(err)
		os.Exit(1)
	}
}
