package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/bitomia/realm/cmd/log"
	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/drivers"
)

var (
	rootCmd = &cobra.Command{}
	cfg     *config.Config

	// version is set at build time via -ldflags "-X main.version=..."
	version = "dev"
)

func main() {
	drivers.RegisterStdDrivers()

	rootCmd.Use = "realm"
	rootCmd.Short = "Realm command-line interface"
	rootCmd.Version = version
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if cmd == rootCmd && len(args) == 0 {
			cmd.Help()
			os.Exit(0)
		}

		configFile, _ := cmd.Flags().GetString("config")
		var configError error

		if configFile == "" {
			cfg, configError = config.Init(nil)
		} else {
			cfg, configError = config.Init(&configFile)
		}
		if configError != nil {
			log.Error("Config error: %s", configError)
			os.Exit(1)
		}
	}
	rootCmd.PersistentFlags().String("config", "config.yaml", "Configuration file")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
