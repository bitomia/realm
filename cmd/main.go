package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/bitomia/realm/cmd/log"
	"github.com/bitomia/realm/common"
	"github.com/bitomia/realm/common/config"
	"github.com/bitomia/realm/common/logging"
	_ "github.com/bitomia/realm/drivers"
	"github.com/bitomia/realm/mesh"
)

var (
	rootCmd = &cobra.Command{}
	cfg     *config.Config
)

func Main(cmdName string, projectName string) {
	common.SetNodeContextBuilder(func(nodeName string) common.NodeContext {
		return common.NodeContext{Repository: nil, Capabilities: nil, NodeName: nodeName, RunMode: common.ClientMode}
	})

	rootCmd.Use = cmdName
	rootCmd.Short = fmt.Sprintf("%s %s", projectName, config.GetVersion())
	rootCmd.Version = config.GetVersion()
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if cmd == rootCmd && len(args) == 0 {
			_ = cmd.Help()
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

		logLevel, levelName, validLevel := logging.ResolveLevel(cfg.LogLevel)
		slog.SetLogLoggerLevel(logLevel)
		if !validLevel {
			slog.Warn("Invalid log level, defaulting to info", "level", levelName)
		}

		if err := mesh.Start(cfg); err != nil {
			log.Error("%s", err)
			os.Exit(1)
		}
	}
	rootCmd.PersistentFlags().String("config", "config.yaml", "Configuration file")

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
