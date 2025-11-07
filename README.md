# Realm - Simple container orchestration service

Realm is a simple orchestration service for OCI containers based on containerd. It will run as a cluster with different nodes running *realm daemon* instances.

Managing the cluster can be commanded from command-line interface or using the REST API that each daemon exposes.

## Development setup

Recommended setup is Debian 12 or Windows 11 Pro with Go >=1.24 installed. 

### Windows 11 Pro setup

Install containerd:

```powershell
Enable-WindowsOptionalFeature -Online -FeatureName containers -All
mkdir "c:\Program Files\containerd"
cd "c:\Program Files\containerd"
curl.exe -L https://github.com/containerd/containerd/releases/download/v2.2.1/containerd-2.2.1-windows-amd64.tar.gz -o containerd-windows-amd64.tar.gz
tar.exe xvf .\containerd-windows-amd64.tar.gz -C "c:\Program Files\containerd"
$Path = [Environment]::GetEnvironmentVariable("PATH", "Machine") + [IO.Path]::PathSeparator + "$Env:ProgramFiles\containerd\bin"
[Environment]::SetEnvironmentVariable("Path", $Path, "Machine")
containerd.exe config default | Out-File "c:\Program Files\containerd\config.toml" -Encoding ascii
containerd --register-service
net start containerd
```

### Debian 12 setup

To build realm you will need also to install some ZFS dependencyes from [Debian Bookworm Backports](https://backports.debian.org/Instructions/).

Install backports as follows:

```shell
cat > /etc/apt/sources.list.d/bookworm-backports.list << EOF
deb http://deb.debian.org/debian bookworm-backports main contrib non-free-firmware
EOF
```

Now install the ZFS devel and ansible dependencies:

```shell
apt update
apt install zfsutils-linux libzfslinux-dev ansible -y
```

## Configuration

Realm can be configured using a YAML configuration file (`realm.yaml`) or environment variables.

### Daemon Configuration

The daemon section configures the realm daemon behavior. All fields are optional and have default values:

```yaml
daemon:
  id_path: /var/lib/realm/realm.id          # Path to store daemon unique ID (default: platform-specific)
  cni_path: /opt/cni                        # Path to CNI plugins (default: /opt/cni on Linux)
  volumes_pool: realm_volumes               # Name of ZFS pool for volumes (default: realm_volumes)
  listen_address: 127.0.0.1                 # Address to bind the daemon API (default: 127.0.0.1)
  listen_port: 9000                         # Port to bind the daemon API (default: 9000)
  logs_path: /var/log/realm                 # Path to store daemon logs (default: /var/log/realm on Linux)
  log_format: text                          # Log format: "text" or "json" (default: text)
  containers_log_path: /var/log/realm/containers  # Path to store container logs (default: platform-specific)
  proxy_enabled: false                      # Enable/disable the proxy (default: false)
  local_caddy_url: localhost:2019           # Local Caddy admin URL (default: localhost:2019)
  master_caddy_url: localhost:2019          # Master Caddy admin URL (default: localhost:2019)
  github_registry_token: ""                 # Token for GitHub registry (default: empty)
  containerd_sock: /run/containerd/containerd.sock  # Containerd socket path (default: platform-specific)
  containerd_namespace: realm               # Containerd namespace (default: realm)
  etcd_data_dir: /var/lib/realm/etcd        # etcd data directory (default: /var/lib/realm/etcd on Linux)
  etcd_name: ""                             # etcd member name (default: empty)
  etcd_listen_client_url: http://127.0.0.1:2379   # etcd client URL (default: http://127.0.0.1:2379)
  etcd_listen_peer_url: http://127.0.0.1:2380     # etcd peer URL (default: http://127.0.0.1:2380)
  etcd_initial_cluster: ""                  # etcd initial cluster configuration (default: empty)
  etcd_cluster_state: new                   # etcd cluster state: "new" or "existing" (default: new)
```

#### Environment Variables

Configuration values can be overridden using environment variables with the `REALM_` prefix and underscores for nested values:

```bash
REALM_DAEMON_LOG_FORMAT=json
REALM_DAEMON_LISTEN_PORT=9001
```

## Production setup

### Disable ipv6 in realm nodes

Add the following lines to /etc/sysctl.conf:

```
net.ipv6.conf.all.disable_ipv6 = 1
net.ipv6.conf.default.disable_ipv6 = 1
net.ipv6.conf.lo.disable_ipv6 = 1
```

Then run the following command to apply the changes:

```
sudo sysctl -p
```

Check if ipv6 is disabled:

```
ip a | grep inet6
```

### Enable iptables persistence in master host

```
sudo apt-get install iptables-persistent
```

Save the current iptables rules:

```
sudo netfilter-persistent save
```

### Security

All containers shall not have any capabilities. For example we don't set NET_ADMIN (https://man7.org/linux/man-pages/man7/capabilities.7.html) to prevent containers modifying routing tables what could allow them to have access to other containers outside of its internal network.

