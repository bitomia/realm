package main

import (
	"fmt"

	clientPkg "github.com/bitomia/realm/cmd/client"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var proxyCmd = &cobra.Command{
	Use:                   "proxy",
	Short:                 "Interface with reverse proxy/server",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Realm CLI. Use -h for help.")
	},
}

var getProxyConfig = &cobra.Command{
	Use:                   "get [node] [container]",
	Short:                 "Get reverse proxy configuration for container",
	Args:                  cobra.ExactArgs(2),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := clientPkg.NewClient()
		color.Blue("Getting proxy config for container %s on %s\n", color.CyanString(args[1]), color.CyanString(args[0]))
		if err := client.GetProxyConfig(args[0], args[1]); err != nil {
			color.Red("Error getting proxy config: %v\n", err)
		}
	},
}

func init() {
	proxyCmd.AddCommand(getProxyConfig)
	rootCmd.AddCommand(proxyCmd)
}
