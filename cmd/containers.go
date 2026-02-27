package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	clientPkg "github.com/bitomia/realm/cmd/client"
	"github.com/bitomia/realm/cmd/log"
)

var containersCmd = &cobra.Command{
	Use:                   "containers",
	Aliases:               []string{"c"},
	Short:                 "Interface with containers",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Realm CLI. Use -h for help.")
	},
}

var listContainers = &cobra.Command{
	Use:                   "ls",
	Short:                 "List all available containers",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := clientPkg.NewClient(cfg)
		containersPerNode, err := client.GetAllContainers()
		if err != nil {
			log.Error("Error %v\n", err)
			return
		}
		for node, containers := range containersPerNode {
			color.Blue("Containers in %s\n", color.CyanString(node))
			for _, c := range containers {
				log.Info("- %s\n", color.CyanString(fmt.Sprintf("%v", c)))
			}
		}
	},
}

var getLogs = &cobra.Command{
	Use:                   "logs [node] [container]",
	Short:                 "Get container logs",
	Args:                  cobra.ExactArgs(2),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := clientPkg.NewClient(cfg)
		node := clientPkg.GetNode(cfg, args[0])

		color.Blue("Getting logs for container %s on %s\n", color.CyanString(args[1]), color.CyanString(args[0]))
		if err := client.GetContainerLogs(node.Url, args[1]); err != nil {
			color.Red("Error getting logs: %v\n", err)
		}
	},
}

func init() {
	containersCmd.AddCommand(listContainers)
	containersCmd.AddCommand(getLogs)
	containersCmd.DisableFlagsInUseLine = true
	rootCmd.AddCommand(containersCmd)
}
