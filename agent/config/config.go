package config

import (
	"log"

	configPkg "github.com/bitomia/realm/common/config"
)

var (
	agentConfig *configPkg.Config
)

func Get() *configPkg.Config {
	if agentConfig == nil {
		log.Fatal("Configuration not initialized with config.Init()")
	}
	return agentConfig
}

func Set(cfg *configPkg.Config) {
	agentConfig = cfg
}
