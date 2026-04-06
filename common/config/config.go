package config

import (
	"fmt"
	"log/slog"
	"net"
	"slices"
	"strings"

	"github.com/dominikbraun/graph"

	"github.com/bitomia/realm/common"
)

var BuildGitCommit string

// RegistryAuth holds authentication credentials for a container registry.
// Token OR Username/Password should be set, not both.
type RegistryAuth struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Token    string `json:"token,omitempty"`
}

// RegistryConfig holds configuration for a container registry.
type RegistryConfig struct {
	Host     string       `json:"host"` // Registry host (e.g., "ghcr.io", "docker.io", "registry.example.com:5000")
	Auth     RegistryAuth `json:"auth"`
	Insecure bool         `json:"insecure,omitempty"` // Allow HTTP instead of HTTPS
}

// DaemonConfig holds the configuration for the realm daemon.
// All fields are optional and have platform-specific or sensible defaults.
type DaemonConfig struct {
	// Path to CNI plugins.
	// Default: /usr/lib/cni (Linux) or %ProgramData%\realm\cni (Windows)
	CniPath string `json:"cni_path"`

	// Name of the ZFS pool for container volumes.
	// Default: realm_volumes
	VolumesPool string `json:"volumes_pool"`

	// Enable ZFS for volume management.
	// When true, uses ZFS-based volume implementation.
	// When false, uses directory-based volume implementation.
	// Default: false
	ZFS bool `json:"zfs"`

	// Address to bind the daemon API.
	// Default: 127.0.0.1
	ListenAddress string `json:"listen_address"`

	// Port to bind the daemon API.
	// Default: 9000
	ListenPort int `json:"listen_port"`

	// Log output format.
	// Valid values: "text", "json"
	// Default: text
	LogFormat string `json:"log_format"`

	// Enables or disables the reverse proxy.
	// Default: false
	ProxyEnabled bool `json:"proxy_enabled"`

	// Local Caddy proxy URL.
	// Default: localhost:2019
	LocalCaddyUrl string `json:"local_caddy_url"`

	// Master Caddy proxy URL.
	// Default: localhost:2019
	MasterCaddyUrl string `json:"master_caddy_url"`

	// Registries holds authentication configuration for container registries.
	// Default: empty (anonymous pulls)
	Registries []RegistryConfig `json:"registries,omitempty"`

	// Multicast address for herd communication.
	HerdMcastAddress string `json:"herd_mcast_address"`

	// Containerd socket path.
	// Default: /run/containerd/containerd.sock (Linux) or npipe://./pipe/containerd-containerd (Windows)
	ContainerdSock string `json:"containerd_sock"`

	// Containerd namespace to use.
	// Default: realm
	ContainerdNamespace string `json:"containerd_namespace"`

	// Etcd mode: "server" to run embedded etcd server, "client" to connect to external etcd.
	// Valid values: "server", "client"
	// Default: server
	EtcdMode string `json:"etcd_mode"`

	// Etcd endpoints to connect to when mode is "client".
	// Example: ["http://node1:2379", "http://node2:2379"]
	// Default: empty (uses EtcdListenClientUrl when in server mode)
	EtcdEndpoints []string `json:"etcd_endpoints"`

	// Etcd client URL.
	// Default: http://<main-network-interface-ip>:2379 (auto-detected)
	EtcdListenClientUrl string `json:"etcd_listen_client_url"`

	// Etcd peer URL.
	// Default: http://<main-network-interface-ip>:2380 (auto-detected)
	EtcdListenPeerUrl string `json:"etcd_listen_peer_url"`

	// Expected members of the cluster, and this is how to reach them via peer URLs.
	// Left empty for single-node cluster
	// Default: empty
	EtcdInitialCluster string `json:"etcd_initial_cluster"`
}

type DiscoveryConfig struct {
	MdnsEnabled bool `json:"mdns"`
}

type LoadsConfig map[string]common.LoadConfig

type NetworkConfig struct {
	IPAddress *net.IP
	Iface     *net.Interface
}

type Config struct {
	// Path to store realm data (ID file, etcd data, logs, mesh configs...).
	// Default: /var/lib/realm (Linux) or %ProgramData%\realm (Windows)
	DataPath string `json:"data_path"`

	// Client config
	Nodes     map[string]*common.NodeConfig `json:"nodes"`
	Discovery DiscoveryConfig               `json:"discovery"`

	// Daemon config
	Daemon DaemonConfig `json:"daemon"`
	Loads  LoadsConfig  `json:"loads"`

	// Autoconfigured network Config
	NetworkConfig NetworkConfig `json:"-"`

	// Processed state (populated during Init)
	processedNodes map[string]*common.Node     `json:"-"`
	processedLoads map[string]*common.Load     `json:"-"`
	loadsGraph     graph.Graph[string, string] `json:"-"`
}

func GetVersion() string {
	return BuildGitCommit
}

func InitFromBuffer(buffer string) (*Config, error) {
	reader := strings.NewReader(buffer)
	cfg, err := readConfigFromReader(reader)

	if err != nil {
		return nil, fmt.Errorf("%s", err.Error())
	}

	return cfg, nil
}

// Init reads configuration from file or environment variables.
// If configFilePath is provided, it will be used instead of the default locations.
func Init(configFilePath *string) (*Config, error) {
	var path string
	if configFilePath != nil {
		path = *configFilePath
	}

	cfg, err := readInConfig(path)
	if err != nil {
		return nil, fmt.Errorf("%s", err.Error())
	}
	return cfg, nil
}

func (c *Config) GetNodes(nodesFilter ...string) map[string]*common.Node {
	nodes := make(map[string]*common.Node)
	for _, node := range c.processedNodes {
		if len(nodesFilter) == 0 || slices.Contains(nodesFilter, node.Name) {
			nodes[node.Name] = node
		}
	}
	return nodes
}

func (c *Config) GetLoads(loadsFilter ...string) map[string]*common.Load {
	loads := make(map[string]*common.Load)
	for _, load := range c.processedLoads {
		if len(loadsFilter) == 0 || slices.Contains(loadsFilter, load.Name) {
			loads[load.Name] = load
		}
	}
	return loads
}

func (c *Config) GetLoadsGraph() graph.Graph[string, string] {
	return c.loadsGraph
}

func (c Config) NeedsCloudInit() bool {
	for _, nodeConfig := range c.Nodes {
		if nodeConfig.CloudInit != nil {
			return true
		}
	}
	return false
}

func findNetworkInterface(targetIP net.IP, interfaces []net.Interface) (NetworkConfig, error) {
	for _, iface := range interfaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip != nil && ip.Equal(targetIP) {
				return NetworkConfig{&ip, &iface}, nil
			}
		}
	}

	return NetworkConfig{nil, nil}, fmt.Errorf("no network interface found")
}

func getNetworkConfigFromIP(ipStr string) (NetworkConfig, error) {
	targetIP := net.ParseIP(ipStr)
	if targetIP == nil {
		return NetworkConfig{nil, nil}, fmt.Errorf("invalid IP address: %s", ipStr)
	}

	interfaces, err := net.Interfaces()
	if err != nil {
		return NetworkConfig{nil, nil}, fmt.Errorf("failed to get network interfaces: %w", err)
	}

	return findNetworkInterface(targetIP, interfaces)
}

func autodetectNetworkConfig() NetworkConfig {
	interfaces, err := net.Interfaces()
	if err != nil {
		slog.Warn("Failed to get network interfaces", "error", err)
		return NetworkConfig{nil, nil}
	}

	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		localIP := localAddr.IP

		if netConfig, err := findNetworkInterface(localIP, interfaces); err == nil {
			return netConfig
		} else {
			return NetworkConfig{&localAddr.IP, nil}
		}
	}

	for _, iface := range interfaces {
		// Skip interfaces that are down or loopback
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Return the first non-loopback IPv4 address
			if ip != nil && ip.To4() != nil && !ip.IsLoopback() {
				return NetworkConfig{&ip, &iface}
			}
		}
	}

	return NetworkConfig{nil, nil}
}
