package main

import (
	"github.com/spf13/cobra"
	"os"
	"podcompose/common"
)

var rootCmd = &cobra.Command{
	Use: "agent",
}
var cleanCmd = &cobra.Command{
	Use: "clean",
	Run: func(cmd *cobra.Command, args []string) {
		err := clean()
		if err != nil {
			panic(err)
		}
	},
}

var startCmd = &cobra.Command{
	Use: "start",
	Run: func(cmd *cobra.Command, args []string) {
		err := start()
		if err != nil {
			panic(err)
		}
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(cleanCmd)
}

func clean() error {
	sessionId := os.Getenv(common.AgentSessionID)
	cleaner, err := NewCleaner(sessionId)
	if err != nil {
		return err
	}
	cleaner.clear()
	return nil
}

func start() error {
	sessionId := os.Getenv(common.AgentSessionID)
	runner, err := NewStarter(common.AgentContextPath, sessionId)
	if err != nil {
		return err
	}
	go func() {
		_ = runner.start()
	}()
	if err = runner.startWebServer(); err != nil {
		return err
	}
	return nil
}

// Execute executes the root command.
func main() {
	err := rootCmd.Execute()
	if err != nil {
		panic(err)
	}
}
