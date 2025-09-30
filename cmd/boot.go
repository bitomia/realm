package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/bitomia/realm/internal"
)

var bootCmd = &cobra.Command{
	Use:                   "boot",
	Short:                 "Interface to startup and shutdown cluster",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Realm CLI. Use -h for help.")
	},
}

var startConfig = &cobra.Command{
	Use:                   "start",
	Short:                 "Startup cluster",
	Args:                  cobra.ExactArgs(2),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		// TODO
	},
}

var shutdownConfig = &cobra.Command{
	Use:                   "start",
	Short:                 "Startup cluster",
	Args:                  cobra.ExactArgs(2),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		nodes := internal.GetNodes()

		color.Blue("Getting proxy config for container %s on %s\n", color.CyanString(args[1]), color.CyanString(args[0]))
		if err := client.GetProxyConfig(args[0], args[1]); err != nil {
			color.Red("Error getting proxy config: %v\n", err)
		}
	},
}

func init() {
	bootCmd.AddCommand(startConfig)
	bootCmd.AddCommand(shutdownConfig)
	rootCmd.AddCommand(bootCmd)
}
