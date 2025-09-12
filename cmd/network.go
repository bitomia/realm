package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var networkCmd = &cobra.Command{
	Use:                   "network",
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
		client := NewClient()
		color.Blue("Listing networks...\n")
		client.ListNetworks()
	},
}

var createNetwork = &cobra.Command{
	Use:                   "create [host] [container]",
	Short:                 "Create network",
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client := NewClient()
		color.Blue("Creating network for container %s on %s\n", color.CyanString(args[1]), color.CyanString(args[0]))
		client.CreateNetwork(args[0], args[1])
		color.Green("Successfully created network for container %s\n", color.CyanString(args[1]))
	},
}

var deleteNetwork = &cobra.Command{
	Use:                   "delete [host] [container]",
	Short:                 "Delete network",
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client := NewClient()
		color.Blue("Deleting network for container %s on %s\n", color.CyanString(args[1]), color.CyanString(args[0]))
		client.DeleteNetwork(args[0], args[1])
		color.Green("Successfully deleted network for container %s\n", color.CyanString(args[1]))
	},
}

var repairNetwork = &cobra.Command{
	Use:                   "repair [host] [container]",
	Short:                 "Repair container network configuration",
	DisableFlagsInUseLine: true,
	Args:                  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		client := NewClient()
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
