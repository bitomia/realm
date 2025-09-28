package main

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/bitomia/realm/cmd/internal"
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
		client := internal.NewClient()
		color.Blue("Getting proxy config for container %s on %s\n", color.CyanString(args[1]), color.CyanString(args[0]))
		if err := client.GetProxyConfig(args[0], args[1]); err != nil {
			color.Red("Error getting proxy config: %v\n", err)
		}
	},
}

var setProxy = &cobra.Command{
	Use:                   "set [node] [container] [upstream] --nodes [node1,node2,...]",
	Short:                 "Set up reverse proxy for container",
	Args:                  cobra.ExactArgs(3),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := internal.NewClient()
		nodesStr, _ := cmd.Flags().GetString("nodes")
		httpUpstream, _ := cmd.Flags().GetBool("http")
		httpsUpstream, _ := cmd.Flags().GetBool("https")

		var nodes []string
		if nodesStr != "" {
			nodes = strings.Split(nodesStr, ",")
			// Trim whitespace from each node
			for i, node := range nodes {
				nodes[i] = strings.TrimSpace(node)
			}
		}

		color.Blue("Setting proxy for container %s on %s with upstream %s\n", color.CyanString(args[1]), color.CyanString(args[0]), color.CyanString(args[2]))
		if err := client.SetProxy(args[0], args[1], nodes, args[2], httpUpstream, httpsUpstream); err != nil {
			color.Red("Error setting proxy: %v\n", err)
		} else {
			color.Green("Successfully set proxy for container %s\n", color.CyanString(args[1]))
		}
	},
}

var deleteProxy = &cobra.Command{
	Use:                   "delete [node] [container]",
	Short:                 "Remove reverse proxy for container",
	Args:                  cobra.ExactArgs(2),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := internal.NewClient()
		color.Blue("Deleting proxy for container %s on %s\n", color.CyanString(args[1]), color.CyanString(args[0]))
		if err := client.DeleteProxy(args[0], args[1]); err != nil {
			color.Red("Error deleting proxy: %v\n", err)
		} else {
			color.Green("Successfully deleted proxy for container %s\n", color.CyanString(args[1]))
		}
	},
}

func init() {
	// Add flags for set proxy command
	setProxy.Flags().String("nodes", "", "Comma-separated list of nodes")
	setProxy.Flags().Bool("http", false, "Enable HTTP upstream")
	setProxy.Flags().Bool("https", false, "Enable HTTPS upstream")

	proxyCmd.AddCommand(getProxyConfig)
	proxyCmd.AddCommand(setProxy)
	proxyCmd.AddCommand(deleteProxy)
	rootCmd.AddCommand(proxyCmd)
}
