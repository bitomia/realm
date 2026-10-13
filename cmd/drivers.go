package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	clientPkg "github.com/bitomia/realm/clients/go"
	"github.com/bitomia/realm/cmd/log"
)

func printDrivers(kind string, entries []clientPkg.DriverEntry) {
	log.Info("%s drivers:", kind)

	if len(entries) == 0 {
		log.Info("  %s", color.YellowString("none"))
		return
	}

	for _, entry := range entries {
		if entry.GuestMode != nil && *entry.GuestMode {
			log.Info("  - %s [%s]", color.CyanString(entry.ID), "guest mode")
		} else {
			log.Info("  - %s", color.CyanString(entry.ID))
		}
	}
}

var driversCmd = &cobra.Command{
	Use:                   "drivers",
	Aliases:               []string{"d"},
	Short:                 "List registered drivers",
	DisableFlagsInUseLine: true,
	PersistentPreRun:      func(cmd *cobra.Command, args []string) {},
	Run: func(cmd *cobra.Command, args []string) {
		drivers := clientPkg.GetRegisteredDrivers()

		if asJson, _ := cmd.Flags().GetBool("json"); asJson {
			out, err := json.Marshal(drivers)
			if err != nil {
				log.Fatal("%s", err.Error())
			}
			fmt.Println(string(out))

			return
		}

		printDrivers("Node", drivers.Nodes)
		printDrivers("Load", drivers.Loads)
		printDrivers("Job", drivers.Jobs)
	},
}

func init() {
	driversCmd.Flags().Bool("json", false, "Output as JSON")
	rootCmd.AddCommand(driversCmd)
}
