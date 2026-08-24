package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/bitomia/realm/cmd/log"
	"github.com/bitomia/realm/common"
)

type driverEntry struct {
	ID        string `json:"id"`
	GuestMode *bool  `json:"guest_mode,omitempty"`
}

type registeredDrivers struct {
	Nodes []driverEntry `json:"nodes"`
	Loads []driverEntry `json:"loads"`
	Jobs  []driverEntry `json:"jobs"`
}

func collectDrivers() registeredDrivers {
	drivers := registeredDrivers{
		Nodes: []driverEntry{},
		Loads: []driverEntry{},
		Jobs:  []driverEntry{},
	}

	for _, info := range common.RegisteredNodeDrivers() {
		guestMode := info.GuestMode
		drivers.Nodes = append(drivers.Nodes, driverEntry{ID: string(info.ID), GuestMode: &guestMode})
	}
	for _, info := range common.RegisteredLoadDrivers() {
		drivers.Loads = append(drivers.Loads, driverEntry{ID: string(info.ID)})
	}
	for _, info := range common.RegisteredJobDrivers() {
		drivers.Jobs = append(drivers.Jobs, driverEntry{ID: string(info.ID)})
	}

	return drivers
}

func printDrivers(kind string, entries []driverEntry) {
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
		drivers := collectDrivers()

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
