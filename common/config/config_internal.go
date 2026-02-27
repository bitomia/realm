package config

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"maps"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/expr-lang/expr"
	"github.com/go-viper/mapstructure/v2"
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

func setDefaults(networkConfig NetworkConfig) {
	// Set platform-specific default paths
	var dataPath, containerdSock, cniPath string
	if runtime.GOOS == "windows" {
		// Windows default paths
		programData := os.Getenv("ProgramData")
		if programData == "" {
			programData = "C:\\ProgramData"
		}
		programFiles := os.Getenv("ProgramFiles")
		if programFiles == "" {
			programFiles = "C:\\Program Files"
		}
		dataPath = filepath.Join(programData, "realm")
		containerdSock = "npipe:////./pipe/containerd-containerd"
		cniPath = filepath.Join(programFiles, "containerd", "cni", "bin")
	} else {
		// Linux/Unix default paths
		dataPath = "/var/lib/realm"
		containerdSock = "/run/containerd/containerd.sock"
		cniPath = "/usr/lib/cni"
	}

	etcdListenIPAdddress := "127.0.0.1"
	if networkConfig.IPAddress != nil {
		etcdListenIPAdddress = networkConfig.IPAddress.String()
	}

	viper.SetDefault("daemon.data_path", dataPath)
	viper.SetDefault("daemon.cni_path", cniPath)
	viper.SetDefault("daemon.volumes_pool", "realm_volumes")
	viper.SetDefault("daemon.listen_address", "127.0.0.1")
	viper.SetDefault("daemon.listen_port", "9000")
	viper.SetDefault("daemon.log_format", "text")
	viper.SetDefault("daemon.proxy_enabled", false)
	viper.SetDefault("daemon.local_caddy_url", "localhost:2019")
	viper.SetDefault("daemon.master_caddy_url", "localhost:2019")
	viper.SetDefault("daemon.containerd_sock", containerdSock)
	viper.SetDefault("daemon.containerd_namespace", "realm")
	viper.SetDefault("daemon.etcd_mode", "server")
	viper.SetDefault("daemon.etcd_endpoints", []string{})
	viper.SetDefault("daemon.etcd_name", "")
	viper.SetDefault("daemon.etcd_listen_client_url", fmt.Sprintf("http://%s:2379", etcdListenIPAdddress))
	viper.SetDefault("daemon.etcd_listen_peer_url", fmt.Sprintf("http://%s:2380", etcdListenIPAdddress))
	viper.SetDefault("daemon.etcd_initial_cluster", "")
	viper.SetDefault("daemon.etcd_cluster_state", "new")
}

func readInConfig(configFilePath string) (*Config, error) {
	return readConfig(func() (*Config, error) {
		var config Config
		if err := viper.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
				return &config, err
			} else {
				log.Println("Config file not found. Continuing using default configuration.")
			}
		}

		err := viper.Unmarshal(&config, func(c *mapstructure.DecoderConfig) {
			c.TagName = "json"
		})

		return &config, err
	}, configFilePath)
}

func readConfigFromReader(in io.Reader) (*Config, error) {
	return readConfig(func() (*Config, error) {
		var config Config
		err := viper.ReadConfig(in)
		if err == nil {
			err = viper.Unmarshal(&config, func(c *mapstructure.DecoderConfig) {
				c.TagName = "json"
			})
		}
		return &config, err
	}, "")
}

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

func readConfig(unmarshall func() (*Config, error), configFilePath string) (*Config, error) {
	networkConfig := autodetectNetworkConfig()
	setDefaults(networkConfig)

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
		viper.SetConfigName("config")
	}

	config, err := unmarshall()
	if err != nil {
		return nil, err
	}

	// If listen_address is configured with a specific IP (not 127.0.0.1 or 0.0.0.0), use it to find the network interface
	if viper.IsSet("daemon.listen_address") && config.Daemon.ListenAddress != "127.0.0.1" && config.Daemon.ListenAddress != "0.0.0.0" {
		configuredNetworkConfig, err := getNetworkConfigFromIP(config.Daemon.ListenAddress)
		if err != nil {
			slog.Warn("Failed to get network config from configured listen_address, using auto-detected network", "listen_address", config.Daemon.ListenAddress, "error", err)
		} else {
			networkConfig = configuredNetworkConfig
			slog.Info("Using network interface from configured listen_address", "ip", networkConfig.IPAddress.String(), "interface", networkConfig.Iface.Name)
		}
	}

	config.NetworkConfig = networkConfig

	if !viper.IsSet("daemon.etcd_listen_client_url") {
		if networkConfig.IPAddress != nil {
			slog.Warn("etcd_listen_client_url not configured, using auto-detected network", "ip", networkConfig.IPAddress.String(), "url", config.Daemon.EtcdListenClientUrl)
		}
	}
	if !viper.IsSet("daemon.etcd_listen_peer_url") {
		if networkConfig.IPAddress != nil {
			slog.Warn("etcd_listen_peer_url not configured, using auto-detected network", "ip", networkConfig.IPAddress, "url", config.Daemon.EtcdListenPeerUrl)
		}
	}

	// Populate node names from map keys
	for nodeName, node := range config.Nodes {
		node.Name = nodeName
		if len(node.Driver) == 0 {
			return nil, fmt.Errorf("driver required for node '%s'", nodeName)
		} else {
			driver, err := common.BuildNodeDriver(common.NodeDriverConfig{Driver: node.Driver, DriverConfig: node.DriverConfig})
			if err != nil {
				return nil, fmt.Errorf("Error building node driver '%s': %s", nodeName, err.Error())
			}
			newNodeConfig(nodeName, node, driver)
		}
	}

	// Check load uniqueness
	l := make(map[string]bool)
	getUniqueValues(l, config.Loads)

	// Create all loads
	allLoadDeps := make(map[string][]string)

	for loadName, loadConfig := range config.Loads {
		if loadConfig.Node == "" {
			return nil, fmt.Errorf("load '%s' with empty node field", loadName)
		}

		env := map[string]any{
			"nodes": slices.Collect(maps.Values(config.Nodes)),
			"selectAny": func(slice []any) any {
				if len(slice) == 0 {
					return nil
				}
				return slice[rand.Intn(len(slice))]
			},
		}
		program, err := expr.Compile(loadConfig.Node, expr.Env(env))
		if err == nil {
			output, err := expr.Run(program, env)
			if err != nil {
				fmt.Printf("%s\n", err)
				panic(err)
			}

			if node, ok := output.(*common.NodeConfig); ok {
				loadConfig.Node = node.Name
			} else {
				return nil, fmt.Errorf("node '%s' evaluated as expression doesn't return a node", loadConfig.Node)
			}
		} else if !strings.Contains(err.Error(), "unknown name") {
			panic(err)
		}

		node, exists := config.Nodes[loadConfig.Node]
		if !exists {
			return nil, fmt.Errorf("node '%s' referenced by load '%s' does not exist", loadConfig.Node, loadName)
		}
		if len(loadConfig.Driver) == 0 {
			return nil, fmt.Errorf("driver required for load '%s'", loadName)
		}
		driver, err := common.BuildLoadDriver(common.LoadDriverConfig{Driver: loadConfig.Driver, DriverConfig: loadConfig.DriverConfig})
		if err != nil {
			return nil, err
		}

		newLoadConfig(loadName, node, driver)
		allLoadDeps[loadName] = loadConfig.DependsOn
	}

	// Traverse all loads and build a DAG
	for loadName, load := range loadsConfig {
		for _, depLoad := range allLoadDeps[loadName] {
			loads, exist := loadsConfig[depLoad]
			if !exist {
				log.Fatalf("dependency node '%s' not exists", depLoad)
			}
			load.DependsOn = append(load.DependsOn, loads)
		}
	}

	// Check for cycles in the dependency graph
	if err := checkForCycles(loadsConfig); err != nil {
		return nil, err
	}

	// Build the loads graph
	if err = newLoadsConfigGraph(); err != nil {
		return nil, err
	}

	// Create chain loads
	for _, load := range loadsConfig {
		load.UpdateLoadChains(loadsConfigGraph, loadsConfig)
	}

	// Populate instance fields
	config.processedNodes = make(map[string]*common.Node, len(nodesConfig))
	for k, v := range nodesConfig {
		config.processedNodes[k] = v
	}
	config.processedLoads = make(map[string]*common.Load, len(loadsConfig))
	for k, v := range loadsConfig {
		config.processedLoads[k] = v
	}
	config.loadsGraph = loadsConfigGraph

	return config, nil
}
