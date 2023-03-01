package main

import (
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"os"
	"podcompose/config"
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
			handleError(err)
			if debug {
				_ = os.Setenv("TPC_DEBUG", "true")
				InitLogger()
			}
			autoStart, err := cmd.Flags().GetBool("autoStart")
			handleError(err)
			bootInDocker, err := cmd.Flags().GetBool("bootInDocker")
			handleError(err)
			configDumpFile, err := cmd.Flags().GetString("configDumpFile")
			handleError(err)
			composeConfig, err := rootCmd.PersistentFlags().GetString("fromConfigJson")
			handleError(err)
			contextPath, err := cmd.Flags().GetString("path")
			handleError(err)
			name, err := cmd.Flags().GetString("name")
			handleError(err)
			if composeConfig != "" {
				handleError(config.SetConfigJson(composeConfig))
			}
			start := NewStartCmd(contextPath, name)
			handleError(start.Start(autoStart, configDumpFile, bootInDocker))
		},
	}
	wdPath, _ := os.Getwd()
	startCmd.Flags().Bool("debug", false, "debug mode")
	startCmd.Flags().Bool("autoStart", true, "auto start compose")
	startCmd.Flags().Bool("bootInDocker", false, "boot agent in docker")
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
	rootCmd.PersistentFlags().String("fromConfigJson", "", "compose config json")
	err := rootCmd.Execute()
	handleError(err)
}
func handleError(err error) {
	if err != nil {
		zap.L().Sugar().Error(err)
		os.Exit(1)
	}
}
