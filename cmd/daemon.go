package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitomia/realm/daemon"
)

var daemonCmd = &cobra.Command{
	Use:                   "daemon",
	Aliases:               []string{"d"},
	Short:                 "Interface with daemon",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Realm CLI. Use -h for help.")
	},
}

var startDaemon = &cobra.Command{
	Use:                   "start",
	Short:                 "Start a daemon",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		purgeDB, _ := cmd.Flags().GetBool("purge-db")
		daemon.Start(cfg, purgeDB, nil)
	},
}

func init() {
	startDaemon.Flags().StringP("config", "c", "", "Path to configuration file (optional, default: config.yaml in current working directory)")
	startDaemon.Flags().Bool("purge-db", false, "Purge all database contents before starting")
	daemonCmd.AddCommand(startDaemon)
	rootCmd.AddCommand(daemonCmd)
}
