package config

import (
	"strings"
	"sync"

	"github.com/spf13/viper"
)

var BuildGitCommit string

type DaemonConfig struct {
	CniPath             string   `mapstructure:"cni_path"`
	VolumesPool         string   `mapstructure:"volumes_pool"`
	ListenAddress       string   `mapstructure:"listen_address"`
	ListenPort          int      `mapstructure:"listen_port"`
	ContainersLogPath   string   `mapstructure:"containers_log_path"`
	LocalCaddyUrl       string   `mapstructure:"local_caddy_url"`
	MasterCaddyUrl      string   `mapstructure:"master_caddy_url"`
	GitHubRegistryToken string   `mapstructure:"github_registry_token"`
	HerdMcastAddress    string   `mapstructure:"herd_mcast_address"`
	ContainerdSock      string   `mapstructure:"containerd_sock"`
	ContainerdNamespace string   `mapstructure:"containerd_namespace"`
	EtcdEndpoints       []string `mapstructure:"etcd_endpoints"`
}

type ClientConfig struct {
}

type Daemon struct {
	Name string `mapstructure:"name"`
	Url  string `mapstructure:"url"`
}

type DiscoveryConfig struct {
	MdnsEnabled bool     `mapstructure:"mdns"`
	Daemons     []Daemon `mapstructure:"daemons"`
}

type Config struct {
	Daemon    DaemonConfig    `mapstructure:"daemon"`
	Client    ClientConfig    `mapstructure:"client"`
	Discovery DiscoveryConfig `mapstructure:"discovery"`
}

var (
	config *Config = nil
	err    error   = nil
	once   sync.Once
)

// Get reads configuration once from file or environment variables.
func Get() *Config {
	once.Do(func() {
		viper.AddConfigPath(getExeDir())
		viper.SetConfigType("yaml")
		viper.SetConfigName("realm.yaml")

		setDefaults()

		viper.SetEnvPrefix("realm")
		viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

		viper.AutomaticEnv()

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
