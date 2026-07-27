package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
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

func setDefaults() {
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

	viper.SetDefault("data_path", dataPath)
	viper.SetDefault("agent.cni_path", cniPath)
	viper.SetDefault("agent.volumes_pool", "realm_volumes")
	viper.SetDefault("agent.listen_address", "0.0.0.0")
	viper.SetDefault("agent.listen_port", "9000")
	viper.SetDefault("agent.log_format", "text")
	viper.SetDefault("agent.containers", true)
	viper.SetDefault("agent.proxy_enabled", false)
	viper.SetDefault("agent.local_caddy_url", "localhost:2019")
	viper.SetDefault("agent.master_caddy_url", "localhost:2019")
	viper.SetDefault("agent.containerd_sock", containerdSock)
	viper.SetDefault("agent.containerd_namespace", "realm")
	viper.SetDefault("agent.artifacts.auth_required", false)
}

// findConfigFile resolves the config file path using the same logic as viper's config setup.
func findConfigFile(configFilePath string) string {
	if configFilePath != "" {
		if _, err := os.Stat(configFilePath); err == nil {
			return configFilePath
		}
		return ""
	}

	if configFile := viper.GetString("config_file"); configFile != "" {
		if _, err := os.Stat(configFile); err == nil {
			return configFile
		}
		return ""
	}

	if cwd, err := os.Getwd(); err == nil {
		for _, ext := range []string{"yaml", "yml"} {
			path := filepath.Join(cwd, "config."+ext)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
	}

	return ""
}

func unmarshalConfigHandler(in io.Reader) (*Config, error) {
	var config Config
	if in == nil {
		if err := viper.ReadInConfig(); err != nil {
			var notFound viper.ConfigFileNotFoundError
			if !errors.As(err, &notFound) && !errors.Is(err, fs.ErrNotExist) {
				return &config, err
			}
		}
	} else {
		if err := viper.ReadConfig(in); err != nil {
			return &config, err
		}
	}
	err := viper.UnmarshalExact(&config, func(c *mapstructure.DecoderConfig) {
		c.TagName = "json"
	})
	return &config, err
}

func readInConfig(configFilePath string) (*Config, error) {
	cfgPath := findConfigFile(configFilePath)
	if cfgPath == "" {
		slog.Warn("Config file not found: using default configuration.")
		return readConfig(unmarshalConfigHandler, nil, configFilePath)
	}

	absPath, err := filepath.Abs(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path %s: %w", cfgPath, err)
	}
	f, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", absPath, err)
	}
	defer f.Close()

	// Resolve inclusions and expand env variables
	data, err := resolveConfig(f, filepath.Dir(absPath))
	if err != nil {
		return nil, err
	}

	return readConfig(unmarshalConfigHandler, bytes.NewBuffer(data), configFilePath)
}

func readConfigFromReader(in io.Reader) (*Config, error) {
	ex, err := os.Executable()
	if err != nil {
		return nil, err
	}
	exPath := filepath.Dir(ex)

	// Resolve inclusions and expand env variables
	data, err := resolveConfig(in, exPath)
	if err != nil {
		return nil, err
	}

	return readConfig(unmarshalConfigHandler, bytes.NewBuffer(data), "")
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

func readConfig(unmarshall func(in io.Reader) (*Config, error), in io.Reader, configFilePath string) (*Config, error) {

	setDefaults()

	viper.AutomaticEnv()
	viper.SetEnvPrefix("realm")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetConfigType("yaml")

	// Priority: command-line flag > environment variable > default
	if configFilePath != "" {
		viper.SetConfigFile(configFilePath)
	} else if configFile := viper.GetString("config_file"); configFile != "" {
		viper.SetConfigFile(configFile)
	} else {
		if cwd, err := os.Getwd(); err == nil {
			viper.AddConfigPath(cwd)
		}
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	config, err := unmarshall(in)
	if err != nil {
		return nil, err
	}

	networkConfig := autodetectNetworkConfig()
	// If listen_address is configured with a specific IP (not 127.0.0.1 or 0.0.0.0), use it to find the network interface
	if viper.IsSet("agent.listen_address") && config.Agent.ListenAddress != "127.0.0.1" && config.Agent.ListenAddress != "0.0.0.0" {
		configuredNetworkConfig, err := getNetworkConfigFromIP(config.Agent.ListenAddress)
		if err != nil {
			slog.Warn("Failed to get network config from configured listen_address, using auto-detected network", "listen_address", config.Agent.ListenAddress, "error", err)
		} else {
			networkConfig = configuredNetworkConfig
			slog.Info("Using network interface from configured listen_address", "ip", networkConfig.IPAddress.String(), "interface", networkConfig.Iface.Name)
		}
	}
	config.NetworkConfig = networkConfig

	// Populate node names from map keys
	for nodeName, node := range config.Nodes {
		node.Name = nodeName
		if len(node.Driver) == 0 {
			return nil, fmt.Errorf("driver required for node '%s'", nodeName)
		} else {
			driver, err := common.BuildNodeDriver(common.NewNodeContext(nodeName), common.NodeDriverConfig{Driver: node.Driver, DriverConfig: node.DriverConfig})
			if err != nil {
				return nil, fmt.Errorf("error building node driver '%s': %s", nodeName, err.Error())
			}
			if _, err := newNodeConfig(nodeName, node, driver); err != nil {
				return nil, err
			}
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

		node, exists := nodesConfig[loadConfig.Node]
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

		if _, err := newLoadConfig(loadName, node, driver); err != nil {
			return nil, err
		}
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
	if err := newLoadsConfigGraph(); err != nil {
		return nil, err
	}

	// Create chain loads
	for _, load := range loadsConfig {
		if err := load.UpdateLoadChains(loadsConfigGraph, loadsConfig); err != nil {
			return nil, err
		}
	}

	// Populate instance fields
	config.processedNodes = make(map[string]*common.Node, len(nodesConfig))
	maps.Copy(config.processedNodes, nodesConfig)

	config.processedLoads = make(map[string]*common.Load, len(loadsConfig))
	maps.Copy(config.processedLoads, loadsConfig)

	config.loadsGraph = loadsConfigGraph

	return config, nil
}
