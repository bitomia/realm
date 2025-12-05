package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/bitomia/realm/cmd/log"
	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/drivers"
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
		var configError error = nil
		if configFile == "" {
			configError = config.Init(nil)
		} else {
			configError = config.Init(&configFile)
		}
		if configError != nil {
			log.Error("Config error: %s", configError)
			os.Exit(1)
		}
	}
	rootCmd.PersistentFlags().String("config", "realm.yaml", "Configuration file")

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
