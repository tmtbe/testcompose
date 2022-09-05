package main

import (
	"github.com/spf13/cobra"
	"os"
	"podcompose/common"
)

var rootCmd = &cobra.Command{
	Use: "agent",
	Run: func(cmd *cobra.Command, args []string) {
		err := start()
		if err != nil {
			panic(err)
		}
	},
}

func start() error {
	sessionId := os.Getenv(common.AgentSessionID)
	runner, err := NewRunner(common.AgentContextPath, sessionId)
	if err != nil {
		return err
	}
	go func() {
		runner.start()
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
