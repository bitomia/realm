package config

import (
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/spf13/viper"

	"github.com/bitomia/realm/internal"
	"github.com/bitomia/realm/internal/loads"
	"github.com/bitomia/realm/internal/loads/drivers"
	"github.com/bitomia/realm/internal/node"
)

var BuildGitCommit string

type DaemonConfig struct {
	CniPath             string            `mapstructure:"cni_path"`
	VolumesPool         string            `mapstructure:"volumes_pool"`
	ListenAddress       string            `mapstructure:"listen_address"`
	ListenPort          int               `mapstructure:"listen_port"`
	LogsPath            internal.LogsPath `mapstructure:"logs_path"`
	ContainersLogPath   string            `mapstructure:"containers_log_path"`
	LocalCaddyUrl       string            `mapstructure:"local_caddy_url"`
	MasterCaddyUrl      string            `mapstructure:"master_caddy_url"`
	GitHubRegistryToken string            `mapstructure:"github_registry_token"`
	HerdMcastAddress    string            `mapstructure:"herd_mcast_address"`
	ContainerdSock      string            `mapstructure:"containerd_sock"`
	ContainerdNamespace string            `mapstructure:"containerd_namespace"`
	EtcdDataDir         string            `mapstructure:"etcd_data_dir"`
	EtcdName            string            `mapstructure:"etcd_name"`
	EtcdListenClientUrl string            `mapstructure:"etcd_listen_client_url"`
	EtcdListenPeerUrl   string            `mapstructure:"etcd_listen_peer_url"`
	EtcdInitialCluster  string            `mapstructure:"etcd_initial_cluster"`
	EtcdClusterState    string            `mapstructure:"etcd_cluster_state"`
}

type DiscoveryConfig struct {
	MdnsEnabled bool `mapstructure:"mdns"`
}

type Config struct {
	// Client config
	Nodes     map[string]*node.Node `mapstructure:"nodes"`
	Discovery DiscoveryConfig       `mapstructure:"discovery"`

	// Daemon config
	Daemon DaemonConfig `mapstructure:"daemon"`
	Loads  LoadsConfig  `mapstructure:"loads"`
}

var (
	config *Config = nil
	err    error   = nil
	once   sync.Once
)

func resetConfig() {
	config = nil
	err = nil
	once = sync.Once{}
}

func getUniqueValues[T any](nodes map[string]bool, values map[string]T) {
	for nodeName := range values {
		if _, exists := nodes[nodeName]; exists {
			log.Fatalf("duplicated node name: %s", nodeName)
		}
		nodes[nodeName] = true
	}
}

func detectCycle(load *loads.Load, visited map[*loads.Load]bool, recStack map[*loads.Load]bool, path []string) error {
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

func checkForCycles(l map[string]*loads.Load) error {
	visited := make(map[*loads.Load]bool)
	recStack := make(map[*loads.Load]bool)

	for _, node := range l {
		if !visited[node] {
			if err := detectCycle(node, visited, recStack, []string{}); err != nil {
				return err
			}
		}
	}
	return nil
}

func readConfig(unmarshall func() (*Config, error)) error {
	setDefaults()

	viper.AutomaticEnv()
	viper.AddConfigPath(getExeDir())
	viper.SetConfigType("yaml")
	viper.SetEnvPrefix("realm")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetConfigName("realm")

	config, err := unmarshall()
	if err != nil {
		return err
	}

	// Populate node names from map keys
	for nodeName, node := range config.Nodes {
		node.Name = nodeName
	}

	// Check load uniqueness
	loads := make(map[string]bool)
	getUniqueValues(loads, config.Loads.Containers)
	getUniqueValues(loads, config.Loads.Processes)

	// Create all loads
	allDeps := make(map[string][]string)
	for containerName, containerConfig := range config.Loads.Containers {
		node, exists := config.Nodes[containerConfig.Node]
		if !exists {
			return fmt.Errorf("node '%s' referenced by container '%s' does not exist", containerConfig.Node, containerName)
		}

		driver, err := drivers.NewContainerDriverFromConfig(containerConfig)
		if err != nil {
			return err
		}

		config.Loads.newLoad(containerName, node, driver)
		allDeps[containerName] = containerConfig.DependsOn
	}
	for procesName, processConfig := range config.Loads.Processes {
		node, exists := config.Nodes[processConfig.Node]
		if !exists {
			return fmt.Errorf("node '%s' referenced by container '%s' does not exist", processConfig.Node, procesName)
		}

		driver, err := drivers.NewProcessDriverFromConfig(processConfig)
		if err != nil {
			return err
		}

		config.Loads.newLoad(procesName, node, driver)
		allDeps[procesName] = processConfig.DependsOn
	}

	// Traverse all loads and build a DAG
	for loadName, load := range config.Loads.loads {
		for _, depLoad := range allDeps[loadName] {
			loads, exist := config.Loads.loads[depLoad]
			if !exist {
				log.Fatalf("dependency node '%s' not exists", depLoad)
			}
			load.DependsOn = append(load.DependsOn, loads)
		}
	}

	// Check for cycles in the dependency graph
	if err := checkForCycles(config.Loads.loads); err != nil {
		return err
	}

	return nil
}

func readInConfig() error {
	return readConfig(func() (*Config, error) {
		if err = viper.ReadInConfig(); err == nil {
			err = viper.Unmarshal(&config)
		}
		return config, err
	})
}

func readConfigFromReader(in io.Reader) error {
	return readConfig(func() (*Config, error) {
		if err = viper.ReadConfig(in); err == nil {
			err = viper.Unmarshal(&config)
		}
		return config, err
	})
}

// Get reads configuration once from file or environment variables.
func Get() *Config {
	once.Do(func() {
		err := readInConfig()
		if err != nil {
			log.Fatal(err.Error())
		}
	})
	return config
}

func GetError() error {
	return err
}

func GetVersion() string {
	return BuildGitCommit
}
