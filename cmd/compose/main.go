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
				common.AgentAutoRemove = false
			}
			contextPath, err := cmd.Flags().GetString("path")
			handleError(err)
			name, err := cmd.Flags().GetString("name")
			handleError(err)
			start := NewStartCmd(contextPath, name)
			handleError(start.Start())
		},
	}
	wdPath, _ := os.Getwd()
	startCmd.Flags().Bool("debug", false, "debug mode")
	startCmd.Flags().StringP("path", "p", wdPath, "context path, normal is $PWD")
	startCmd.Flags().StringP("name", "n", "", "set the test compose name, normal is uuid")
	stopCmd := &cobra.Command{
		Use: "stop",
		Run: func(cmd *cobra.Command, args []string) {
			stop, err := NewStopCmd(args)
			handleError(err)
			handleError(stop.Stop())
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
	rootCmd.AddCommand(stopCmd)
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
