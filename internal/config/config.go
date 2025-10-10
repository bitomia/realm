package config

import (
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

var BuildGitCommit string

type DaemonConfig struct {
	CniPath             string `mapstructure:"cni_path"`
	VolumesPool         string `mapstructure:"volumes_pool"`
	ListenAddress       string `mapstructure:"listen_address"`
	ListenPort          int    `mapstructure:"listen_port"`
	ContainersLogPath   string `mapstructure:"containers_log_path"`
	LocalCaddyUrl       string `mapstructure:"local_caddy_url"`
	MasterCaddyUrl      string `mapstructure:"master_caddy_url"`
	GitHubRegistryToken string `mapstructure:"github_registry_token"`
	HerdMcastAddress    string `mapstructure:"herd_mcast_address"`
	ContainerdSock      string `mapstructure:"containerd_sock"`
	ContainerdNamespace string `mapstructure:"containerd_namespace"`
	EtcdDataDir         string `mapstructure:"etcd_data_dir"`
	EtcdName            string `mapstructure:"etcd_name"`
	EtcdListenClientUrl string `mapstructure:"etcd_listen_client_url"`
	EtcdListenPeerUrl   string `mapstructure:"etcd_listen_peer_url"`
	EtcdInitialCluster  string `mapstructure:"etcd_initial_cluster"`
	EtcdClusterState    string `mapstructure:"etcd_cluster_state"`
}

type DiscoveryConfig struct {
	MdnsEnabled bool `mapstructure:"mdns"`
}

type Config struct {
	// Client config
	Nodes     map[string]*Node `mapstructure:"nodes"`
	Discovery DiscoveryConfig  `mapstructure:"discovery"`

	// Daemon config
	Daemon DaemonConfig `mapstructure:"daemon"`
	Loads  LoadsConfig  `mapstructure:"loads"`
}

var (
	config *Config = nil
	err    error   = nil
	once   sync.Once
)

func getUniqueValues[T any](nodes map[string]bool, values map[string]T) {
	for nodeName := range values {
		if _, exists := nodes[nodeName]; exists {
			log.Fatal(fmt.Sprintf("duplicated node name: %s", nodeName))
		}
		nodes[nodeName] = true
	}
}

func detectCycle(node *Load, visited map[*Load]bool, recStack map[*Load]bool, path []string) error {
	visited[node] = true
	recStack[node] = true
	path = append(path, node.name)

	for _, dep := range node.dependsOn {
		if !visited[dep] {
			if err := detectCycle(dep, visited, recStack, path); err != nil {
				return err
			}
		} else if recStack[dep] {
			// Found a cycle
			cycleStart := -1
			for i, name := range path {
				if name == dep.name {
					cycleStart = i
					break
				}
			}
			cyclePath := append(path[cycleStart:], dep.name)
			return fmt.Errorf("cycle detected in dependencies: %s", strings.Join(cyclePath, " -> "))
		}
	}

	recStack[node] = false
	return nil
}

func checkForCycles(loadNodes map[string]*Load) error {
	visited := make(map[*Load]bool)
	recStack := make(map[*Load]bool)

	for _, node := range loadNodes {
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

	// Check load node uniqueness
	loadNodes := make(map[string]bool)
	getUniqueValues(loadNodes, config.Loads.Containers)
	getUniqueValues(loadNodes, config.Loads.Processes)

	// Create all load nodes
	allDeps := make(map[string][]string)
	for containerName, containerConfig := range config.Loads.Containers {
		node, exists := config.Nodes[containerConfig.Node]
		if !exists {
			return fmt.Errorf("node '%s' referenced by container '%s' does not exist", containerConfig.Node, containerName)
		}
		config.Loads.newLoadNode(containerName, node, NewContainerDriverFromConfig(containerConfig))
		allDeps[containerName] = containerConfig.DependsOn
	}
	for procesName, processConfig := range config.Loads.Processes {
		node, exists := config.Nodes[processConfig.Node]
		if !exists {
			return fmt.Errorf("node '%s' referenced by container '%s' does not exist", processConfig.Node, procesName)
		}
		config.Loads.newLoadNode(procesName, node, NewProcessDriverFromConfig(processConfig))
		allDeps[procesName] = processConfig.DependsOn
	}

	// Traverse all load nodes and build a DAG
	for loadNodeName, loadNode := range config.Loads.loads {
		for _, depLoadNode := range allDeps[loadNodeName] {
			loadNodes, exist := config.Loads.loads[depLoadNode]
			if !exist {
				log.Fatal(fmt.Sprintf("dependency node '%s' not exists", depLoadNode))
			}
			loadNode.dependsOn = append(loadNode.dependsOn, loadNodes)
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
