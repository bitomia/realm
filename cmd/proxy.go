package main

import (
	"fmt"
	"strings"

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
	Use:                   "get [host] [container]",
	Short:                 "Get reverse proxy configuration for container",
	Args:                  cobra.ExactArgs(2),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := NewClient()
		color.Blue("Getting proxy config for container %s on %s\n", color.CyanString(args[1]), color.CyanString(args[0]))
		if err := client.GetProxyConfig(args[0], args[1]); err != nil {
			color.Red("Error getting proxy config: %v\n", err)
		}
	},
}

var setProxy = &cobra.Command{
	Use:                   "set [host] [container] [upstream] --hosts [host1,host2,...]",
	Short:                 "Set up reverse proxy for container",
	Args:                  cobra.ExactArgs(3),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := NewClient()
		hostsStr, _ := cmd.Flags().GetString("hosts")
		httpUpstream, _ := cmd.Flags().GetBool("http")
		httpsUpstream, _ := cmd.Flags().GetBool("https")

		var hosts []string
		if hostsStr != "" {
			hosts = strings.Split(hostsStr, ",")
			// Trim whitespace from each host
			for i, host := range hosts {
				hosts[i] = strings.TrimSpace(host)
			}
		}

		color.Blue("Setting proxy for container %s on %s with upstream %s\n", color.CyanString(args[1]), color.CyanString(args[0]), color.CyanString(args[2]))
		if err := client.SetProxy(args[0], args[1], hosts, args[2], httpUpstream, httpsUpstream); err != nil {
			color.Red("Error setting proxy: %v\n", err)
		} else {
			color.Green("Successfully set proxy for container %s\n", color.CyanString(args[1]))
		}
	},
}

var deleteProxy = &cobra.Command{
	Use:                   "delete [host] [container]",
	Short:                 "Remove reverse proxy for container",
	Args:                  cobra.ExactArgs(2),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := NewClient()
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
	setProxy.Flags().String("hosts", "", "Comma-separated list of hostnames")
	setProxy.Flags().Bool("http", false, "Enable HTTP upstream")
	setProxy.Flags().Bool("https", false, "Enable HTTPS upstream")

	proxyCmd.AddCommand(getProxyConfig)
	proxyCmd.AddCommand(setProxy)
	proxyCmd.AddCommand(deleteProxy)
	rootCmd.AddCommand(proxyCmd)
}
