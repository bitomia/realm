package config

import (
	"fmt"
	"log"
	"strings"

	"github.com/bitomia/realm/common"
)

var BuildGitCommit string

// DaemonConfig holds the configuration for the realm daemon.
// All fields are optional and have platform-specific or sensible defaults.
type DaemonConfig struct {
	// Path to store daemon unique ID.
	// Default: /var/lib/realm/realm.id (Linux) or %ProgramData%\realm\realm.id (Windows)
	IdPath string `json:"id_path"`

	// Path to CNI plugins.
	// Default: /opt/cni (Linux) or %ProgramData%\realm\cni (Windows)
	CniPath string `json:"cni_path"`

	// Name of the ZFS pool for container volumes.
	// Default: realm_volumes
	VolumesPool string `json:"volumes_pool"`

	// Address to bind the daemon API.
	// Default: 127.0.0.1
	ListenAddress string `json:"listen_address"`

	// Port to bind the daemon API.
	// Default: 9000
	ListenPort int `json:"listen_port"`

	// Path to store daemon logs.
	// Default: /var/log/realm (Linux) or %ProgramData%\realm\logs (Windows)
	LogsPath common.LogsPath `json:"logs_path"`

	// Log output format.
	// Valid values: "text", "json"
	// Default: text
	LogFormat string `json:"log_format"`

	// Path to store container logs.
	// Default: /var/log/realm/containers (Linux) or %ProgramData%\realm\logs\containers (Windows)
	ContainersLogPath string `json:"containers_log_path"`

	// Enables or disables the reverse proxy.
	// Default: false
	ProxyEnabled bool `json:"proxy_enabled"`

	// Local Caddy proxy URL.
	// Default: localhost:2019
	LocalCaddyUrl string `json:"local_caddy_url"`

	// Master Caddy proxy URL.
	// Default: localhost:2019
	MasterCaddyUrl string `json:"master_caddy_url"`

	// Token for GitHub container registry authentication. Used to pull container images.
	// Default: empty
	GitHubRegistryToken string `json:"github_registry_token"`

	// Multicast address for herd communication.
	HerdMcastAddress string `json:"herd_mcast_address"`

	// Containerd socket path.
	// Default: /run/containerd/containerd.sock (Linux) or npipe://./pipe/containerd-containerd (Windows)
	ContainerdSock string `json:"containerd_sock"`

	// Containerd namespace to use.
	// Default: realm
	ContainerdNamespace string `json:"containerd_namespace"`

	// Etcd data directory.
	// Default: /var/lib/realm/etcd (Linux) or %ProgramData%\realm\etcd (Windows)
	EtcdDataDir string `json:"etcd_data_dir"`

	// Etcd member name.
	// Default: empty
	EtcdName string `json:"etcd_name"`

	// Etcd client URL.
	// Default: http://127.0.0.1:2379
	EtcdListenClientUrl string `json:"etcd_listen_client_url"`

	// Etcd peer URL.
	// Default: http://127.0.0.1:2380
	EtcdListenPeerUrl string `json:"etcd_listen_peer_url"`

	// Deprecate
	// Default: empty
	EtcdInitialCluster string `json:"etcd_initial_cluster"`

	// Deprecate
	// Valid values: "new", "existing"
	// Default: new
	EtcdClusterState string `json:"etcd_cluster_state"`
}

type DiscoveryConfig struct {
	MdnsEnabled bool `json:"mdns"`
}

type LoadsConfig map[string]common.LoadConfig

type Config struct {
	// Client config
	Nodes     map[string]*common.Node `json:"nodes"`
	Discovery DiscoveryConfig         `json:"discovery"`

	// Daemon config
	Daemon DaemonConfig `json:"daemon"`
	Loads  LoadsConfig  `json:"loads"`
}

// Get returns the global configuration instance.
//
// This function should be called only after config.Init() or config.InitFromBuffer()
// have successfully initialized the configuration. If the configuration has not been
// initialized,  Get will terminate the program with a fatal log message.
//
// Returns:
//   - *Config: the initialized global configuration.
//
// Panics/Fatal:
//   - Logs a fatal error and exits the program if the configuration has not
//     been initialized before calling Get().
func Get() *Config {
	if config == nil {
		log.Fatal("Configuration not initialized with config.Init()")
	}
	return config
}

func GetVersion() string {
	return BuildGitCommit
}

func ResetConfig() {
	config = nil
}

func InitFromBuffer(buffer string) error {
	if config != nil {
		return fmt.Errorf("Configuration already initialized")
	}

	reader := strings.NewReader(buffer)
	err := readConfigFromReader(reader)
	if err != nil {
		return fmt.Errorf("%s", err.Error())
	}
	return nil
}

// Init reads configuration from file or environment variables.
// If configFilePath is provided, it will be used instead of the default locations.
func Init(configFilePath *string) error {
	if config != nil {
		return fmt.Errorf("Configuration already initialized")
	}

	var path string
	if configFilePath != nil {
		path = *configFilePath
	}

	err := readInConfig(path)
	if err != nil {
		return fmt.Errorf("%s", err.Error())
	}
	return nil
}
