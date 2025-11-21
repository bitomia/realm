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
		configFile, _ := cmd.Flags().GetString("config")
		daemon.Start(configFile)
	},
}

func init() {
	startDaemon.Flags().StringP("config", "c", "", "Path to configuration file (default: realm.yaml in executable directory)")
	daemonCmd.AddCommand(startDaemon)
	rootCmd.AddCommand(daemonCmd)
}
