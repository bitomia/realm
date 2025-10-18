package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/bitomia/realm/cmd/internal"
	"github.com/bitomia/realm/cmd/log"
	"github.com/bitomia/realm/internal/config"
)

var loadsCmd = &cobra.Command{
	Use:                   "loads",
	Aliases:               []string{"l"},
	Short:                 "Interface with loads",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Realm CLI. Use -h for help.")
	},
}

var renderLoadsGraph = &cobra.Command{
	Use:                   "graph [output_file]",
	Short:                 "Render SVG graph of loads and their dependencies",
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

		log.Info("Successfully generated dependency graph: %s\n", outputFile)
	},
}

var prepareLoads = &cobra.Command{
	Use:   "verify",
	Short: "Verify loads and nodes to verify correctness",
	Run: func(cmd *cobra.Command, args []string) {
		cfg := config.Get()
		if cfg == nil {
			log.Error("Failed to load configuration\n")
			return
		}

		client := internal.NewClient()
		for _, load := range cfg.Loads.GetLoads() {
			if err := client.VerifyLoad(load); err != nil {
				log.Fatal("Error verifying load: %s", err.Error())
			}
		}
	},
}

func init() {
	loadsCmd.AddCommand(renderLoadsGraph)
	loadsCmd.AddCommand(prepareLoads)
	loadsCmd.DisableFlagsInUseLine = true
	rootCmd.AddCommand(loadsCmd)
}
