package main

import (
	"fmt"

	"github.com/dominikbraun/graph"
	"github.com/fatih/color"
	"github.com/spf13/cobra"

	clientPkg "github.com/bitomia/realm/cmd/client"
	"github.com/bitomia/realm/cmd/log"
	"github.com/bitomia/realm/common/config"
)

func doPlanLoads(client *clientPkg.Client) error {
	loads := config.GetLoads()
	if len(loads) == 0 {
		return fmt.Errorf("No loads present in config file")
	}
	for _, load := range loads {
		if err := client.PlanLoad(load); err != nil {
			return fmt.Errorf("Error planning load: %s", err.Error())
		}
	}
	return nil
}

var loadsCmd = &cobra.Command{
	Use:                   "loads",
	Aliases:               []string{"l"},
	Short:                 "Interface with loads",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Realm CLI. Use -h for help.")
	},
}

var drawLoadsGraph = &cobra.Command{
	Use:                   "draw [output_file]",
	Short:                 "Create a SVG of the graph loads",
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		outputFile := args[0]

		cfg := config.Get()
		if cfg == nil {
			log.Error("Failed to load configuration\n")
			return
		}

		err := generateSVG(cfg.Loads, outputFile)
		if err != nil {
			log.Error("Error generating graph: %v\n", err)
			return
		}

		log.Info("Successfully generated SVG of the dependency graph: %s", outputFile)
	},
}

var planLoads = &cobra.Command{
	Use:   "plan",
	Short: "Plan loads on nodes",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Get()
		if cfg == nil {
			log.Error("Failed to load configuration\n")
			return
		}

		client := clientPkg.NewClient()
		if err := doPlanLoads(&client); err != nil {
			log.Fatal("Error planning load: %s", err.Error())
		}
		log.Info("Successfully verified loads on cluster")
	},
}

var runLoads = &cobra.Command{
	Use:   "start",
	Short: "Start all the loads into the cluster",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Get()
		if cfg == nil {
			log.Error("Failed to load configuration\n")
			return
		}

		client := clientPkg.NewClient()

		// Plan all loads first
		if err := doPlanLoads(&client); err != nil {
			log.Fatal("Error planning load: %s", err.Error())
		}

		// Start loads
		g, err := NewGraph(cfg.Loads)
		if err != nil {
			log.Fatal("Error building graph: %s", err.Error())
		}

		log.Info("Starting loads")
		loads := config.GetLoads()
		loaded := make(map[string]bool)

		for _, l := range loads {
			var pendingLoads []string

			graph.DFS(g, l.Name, func(value string) bool {
				pendingLoads = append(pendingLoads, l.Name)
				return true
			})

			for i := len(pendingLoads) - 1; i >= 0; i-- {
				load := pendingLoads[i]

				if _, exists := loaded[load]; !exists {
					loaded[load] = true
					loadRun := loads[load]

					log.Info(" -> Running load %s", color.CyanString(loadRun.Name))
					if err := client.StartLoad(loadRun); err != nil {
						log.Fatal("Starting load failed: %s", err.Error())
					}
				}
			}
		}
	},
}

func init() {
	loadsCmd.AddCommand(drawLoadsGraph)
	loadsCmd.AddCommand(planLoads)
	loadsCmd.AddCommand(runLoads)
	loadsCmd.DisableFlagsInUseLine = true
	rootCmd.AddCommand(loadsCmd)
}
