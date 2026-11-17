package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitomia/realm/agent"
)

var agentCmd = &cobra.Command{
	Use:                   "agent",
	Aliases:               []string{"a"},
	Short:                 "Interface with agent",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Realm CLI. Use -h for help.")
	},
}

var startAgent = &cobra.Command{
	Use:                   "start",
	Short:                 "Start a agent",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		agent.Start(cfg, false, nil)
	},
}

func init() {
	startAgent.Flags().StringP("config", "c", "", "Path to configuration file (default: config.yaml)")
	agentCmd.AddCommand(startAgent)
	rootCmd.AddCommand(agentCmd)
}
