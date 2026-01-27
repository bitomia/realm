package main

import (
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	clientPkg "github.com/bitomia/realm/cmd/client"
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
		client := clientPkg.NewClient()
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

func init() {
	networkCmd.AddCommand(listNetworks)
	rootCmd.AddCommand(networkCmd)
}
