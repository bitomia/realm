package config

import (
	"log"

	configPkg "github.com/bitomia/realm/common/config"
)

var (
	daemonConfig *configPkg.Config
)

func Get() *configPkg.Config {
	if daemonConfig == nil {
		log.Fatal("Configuration not initialized with config.Init()")
	}
	return daemonConfig
}

func Set(cfg *configPkg.Config) {
	daemonConfig = cfg
}
