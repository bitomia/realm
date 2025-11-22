package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitomia/realm/daemon"
)

var daemonCmd = &cobra.Command{
	Use:                   "daemon",
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
		daemon.Start()
	},
}

func init() {
	startDaemon.Flags().StringP("config", "c", "", "Path to configuration file (optional, default: realm.yaml in executable directory)")
	daemonCmd.AddCommand(startDaemon)
	rootCmd.AddCommand(daemonCmd)
}
