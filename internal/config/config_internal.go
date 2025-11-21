package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"github.com/spf13/viper"
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
