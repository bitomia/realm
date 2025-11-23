package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bitomia/realm/internal/config"
	"github.com/bitomia/realm/internal/drivers"
)

var rootCmd = &cobra.Command{}

func main() {
	drivers.RegisterStdDrivers()

	rootCmd.Use = "realm"
	rootCmd.Short = "Realm command-line interface"
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if cmd == rootCmd && len(args) == 0 {
			cmd.Help()
			os.Exit(0)
		}

		configFile, _ := cmd.Flags().GetString("config")
		if configFile == "" {
			config.Init(nil)
		} else {
			config.Init(&configFile)
		}
	}
	rootCmd.PersistentFlags().String("config", "realm.yaml", "Configuration file")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
