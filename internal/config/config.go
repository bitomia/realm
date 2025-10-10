package config

import (
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

type Node struct {
	Name string
	Url  string  `mapstructure:"url"`
	MAC  *string `mapstructure:"mac"`
}

type DiscoveryConfig struct {
	MdnsEnabled bool `mapstructure:"mdns"`
}

type Config struct {
	// Client config
	Nodes     map[string]Node `mapstructure:"nodes"`
	Discovery DiscoveryConfig `mapstructure:"discovery"`

	// Daemon config
	Daemon DaemonConfig `mapstructure:"daemon"`
}

var (
	config *Config = nil
	err    error   = nil
	once   sync.Once
)

// Get reads configuration once from file or environment variables.
func Get() *Config {
	once.Do(func() {
		setDefaults()

		viper.AutomaticEnv()
		viper.AddConfigPath(getExeDir())
		viper.SetConfigType("yaml")
		viper.SetEnvPrefix("realm")
		viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		viper.SetConfigName("realm")

		if err = viper.ReadInConfig(); err == nil {
			err = viper.Unmarshal(&config)
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
