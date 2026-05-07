package main

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
		purgeDB, _ := cmd.Flags().GetBool("purge-db")
		agent.Start(cfg, purgeDB, nil)
	},
}

func init() {
	startAgent.Flags().StringP("config", "c", "", "Path to configuration file (optional, default: config.yaml in current working directory)")
	startAgent.Flags().Bool("purge-db", false, "Purge all database contents before starting")
	agentCmd.AddCommand(startAgent)
	rootCmd.AddCommand(agentCmd)
}
