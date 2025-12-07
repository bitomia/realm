package config

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/viper"

	"github.com/bitomia/realm/common"
)

func getExeDir() string {
	execPath, err := os.Executable()
	if err != nil {
		slog.Error(err.Error())
	}

	return filepath.Dir(execPath)
}

func setDefaults() {
	// Set platform-specific default paths
	var logsPath, containersLogPath, etcdDataDir, containerdSock, cniPath, idPath string
	if runtime.GOOS == "windows" {
		// Windows default paths
		programData := os.Getenv("ProgramData")
		if programData == "" {
			programData = "C:\\ProgramData"
		}
		logsPath = filepath.Join(programData, "realm", "logs")
		containersLogPath = filepath.Join(programData, "realm", "logs", "containers")
		etcdDataDir = filepath.Join(programData, "realm", "etcd")
		containerdSock = "npipe:////./pipe/containerd-containerd"
		cniPath = filepath.Join(programData, "realm", "cni")
		idPath = filepath.Join(programData, "realm", "realm.id")
	} else {
		// Linux/Unix default paths
		logsPath = "/var/log/realm"
		containersLogPath = "/var/log/realm/containers"
		etcdDataDir = "/var/lib/realm/etcd"
		containerdSock = "/run/containerd/containerd.sock"
		cniPath = "/opt/cni"
		idPath = "/var/lib/realm/realm.id"
	}

	viper.SetDefault("daemon.id_path", idPath)
	viper.SetDefault("daemon.cni_path", cniPath)
	viper.SetDefault("daemon.volumes_pool", "realm_volumes")
	viper.SetDefault("daemon.listen_address", "127.0.0.1")
	viper.SetDefault("daemon.listen_port", "9000")
	viper.SetDefault("daemon.logs_path", logsPath)
	viper.SetDefault("daemon.log_format", "text")
	viper.SetDefault("daemon.proxy_enabled", false)
	viper.SetDefault("daemon.local_caddy_url", "localhost:2019")
	viper.SetDefault("daemon.master_caddy_url", "localhost:2019")
	viper.SetDefault("daemon.containerd_sock", containerdSock)
	viper.SetDefault("daemon.containerd_namespace", "realm")
	viper.SetDefault("daemon.containers_log_path", containersLogPath)
	viper.SetDefault("daemon.etcd_data_dir", etcdDataDir)
	viper.SetDefault("daemon.etcd_name", "")
	viper.SetDefault("daemon.etcd_listen_client_url", "http://127.0.0.1:2379")
	viper.SetDefault("daemon.etcd_listen_peer_url", "http://127.0.0.1:2380")
	viper.SetDefault("daemon.etcd_initial_cluster", "")
	viper.SetDefault("daemon.etcd_cluster_state", "new")
}

func readInConfig(configFilePath string) error {
	return readConfig(func() (*Config, error) {
		err := viper.ReadInConfig()
		if err == nil {
			err = viper.Unmarshal(&config)
		}
		return config, err
	}, configFilePath)
}

func readConfigFromReader(in io.Reader) error {
	return readConfig(func() (*Config, error) {
		err := viper.ReadConfig(in)
		if err == nil {
			err = viper.Unmarshal(&config)
		}
		return config, err
	}, "")
}

var (
	config *Config = nil
)

func getUniqueValues[T any](nodes map[string]bool, values map[string]T) {
	for nodeName := range values {
		if _, exists := nodes[nodeName]; exists {
			log.Fatalf("duplicated node name: %s", nodeName)
		}
		nodes[nodeName] = true
	}
}

func detectCycle(load *common.Load, visited map[*common.Load]bool, recStack map[*common.Load]bool, path []string) error {
	visited[load] = true
	recStack[load] = true
	path = append(path, load.Name)

	for _, dep := range load.DependsOn {
		if !visited[dep] {
			if err := detectCycle(dep, visited, recStack, path); err != nil {
				return err
			}
		} else if recStack[dep] {
			// Found a cycle
			cycleStart := -1
			for i, name := range path {
				if name == dep.Name {
					cycleStart = i
					break
				}
			}
			cyclePath := append(path[cycleStart:], dep.Name)
			return fmt.Errorf("cycle detected in dependencies: %s", strings.Join(cyclePath, " -> "))
		}
	}

	recStack[load] = false
	return nil
}

func checkForCycles(l map[string]*common.Load) error {
	visited := make(map[*common.Load]bool)
	recStack := make(map[*common.Load]bool)

	for _, node := range l {
		if !visited[node] {
			if err := detectCycle(node, visited, recStack, []string{}); err != nil {
				return err
			}
		}
	}
	return nil
}

func readConfig(unmarshall func() (*Config, error), configFilePath string) error {
	setDefaults()

	viper.AutomaticEnv()
	viper.SetEnvPrefix("realm")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Priority: command-line flag > environment variable > default
	if configFilePath != "" {
		viper.SetConfigFile(configFilePath)
	} else if configFile := viper.GetString("config_file"); configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		viper.AddConfigPath(getExeDir())
		viper.SetConfigType("yaml")
		viper.SetConfigName("realm")
	}

	config, err := unmarshall()
	if err != nil {
		return err
	}

	// Populate node names from map keys
	for nodeName, node := range config.Nodes {
		node.Name = nodeName
	}

	// Check load uniqueness
	l := make(map[string]bool)
	getUniqueValues(l, config.Loads)

	// Create all loads
	allDeps := make(map[string][]string)

	for loadName, loadConfig := range config.Loads {
		if loadConfig.Node == "" {
			return fmt.Errorf("load '%s' has an empty node field", loadName)
		}
		node, exists := config.Nodes[loadConfig.Node]
		if !exists {
			return fmt.Errorf("node '%s' referenced by load '%s' does not exist", loadConfig.Node, loadName)
		}

		driver, err := common.BuildLoadDriver(loadConfig)
		if err != nil {
			return err
		}

		newLoad(loadName, node, driver)
		allDeps[loadName] = loadConfig.DependsOn
	}

	// Traverse all loads and build a DAG
	for loadName, load := range loadsRepository {
		for _, depLoad := range allDeps[loadName] {
			loads, exist := loadsRepository[depLoad]
			if !exist {
				log.Fatalf("dependency node '%s' not exists", depLoad)
			}
			load.DependsOn = append(load.DependsOn, loads)
		}
	}

	// Check for cycles in the dependency graph
	if err := checkForCycles(loadsRepository); err != nil {
		return err
	}

	return nil
}
