package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/bitomia/realm/cmd/internal"
	"github.com/bitomia/realm/cmd/log"
)

var networkCmd = &cobra.Command{
	Use:                   "networks",
	Aliases:               []string{"net"},
	Short:                 "Interface with network",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Realm CLI. Use -h for help.")
	},
}

var listNetworks = &cobra.Command{
	Use:                   "ls",
	Short:                 "List all available network",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		client := internal.NewClient()
		color.Blue("Listing networks...\n")
		networksPerHost, err := client.ListNetworks()

		if err != nil {
			log.Error("Error %v\n", err)
			return
		}
		for host, networks := range networksPerHost {
			color.Blue("Networks in %s\n", color.CyanString(host))
			log.Info("- %s\n", color.CyanString(fmt.Sprintf("%v", networks)))
		}
	},
}

var createNetwork = &cobra.Command{
	Use:                   "create [node] [container]",
	Short:                 "Create network",
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client := internal.NewClient()

		nodes := internal.GetNodes()
		node, exists := nodes[args[0]]

		if !exists {
			log.Fatal("Node %s not found", args[0])
		}

		color.Blue("Creating network for container %s on %s\n", color.CyanString(args[1]), color.CyanString(args[0]))
		if err := client.CreateNetwork(node.Url.String(), args[1]); err != nil {
			log.Error("%s", err.Error())
		} else {
			color.Green("Successfully created network for container %s\n", color.CyanString(args[1]))
		}
	},
}

var deleteNetwork = &cobra.Command{
	Use:                   "delete [node] [container]",
	Short:                 "Delete network",
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client := internal.NewClient()

		nodes := internal.GetNodes()
		node, exists := nodes[args[0]]

		if !exists {
			log.Fatal("Node %s not found", args[0])
		}

		color.Blue("Deleting network for container %s on %s\n", color.CyanString(args[1]), color.CyanString(args[0]))
		if err := client.DeleteNetwork(node.Url.String(), args[1]); err != nil {
			log.Error("%s", err.Error())
		} else {
			color.Green("Successfully deleted network for container %s\n", color.CyanString(args[1]))
		}
	},
}

var repairNetwork = &cobra.Command{
	Use:                   "repair [node] [container]",
	Short:                 "Repair container network configuration",
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client := internal.NewClient()
		color.Blue("Repairing network for container %s on %s\n", color.CyanString(args[1]), color.CyanString(args[0]))
		if err := client.RepairNetwork(args[0], args[1]); err != nil {
			color.Red("Error repairing network: %v\n", err)
		} else {
			color.Green("Successfully repaired network for container %s\n", color.CyanString(args[1]))
		}
	},
}

func init() {
	networkCmd.AddCommand(listNetworks)
	networkCmd.AddCommand(createNetwork)
	networkCmd.AddCommand(deleteNetwork)
	networkCmd.AddCommand(repairNetwork)
	rootCmd.AddCommand(networkCmd)
}
