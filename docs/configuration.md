# Configuration

Realm is configured through a YAML file. By default, Realm looks for a `config.yaml` file in the same directory as the executable. You can specify a custom path using the `--config` flag or the `REALM_CONFIG_FILE` environment variable.

## File Structure

A Realm configuration file has four top-level sections:

```yaml
daemon: # Daemon settings (paths, etcd, registries)
nodes: # Remote nodes to manage
loads: # Workloads to deploy
discovery: # Discovery settings
```

## Root config

| Field       | Type   | Default          | Description                                              |
| ----------- | ------ | ---------------- | -------------------------------------------------------- |
| `data_path` | string | `/var/lib/realm` | Path to store client or daemon data (ID file, etcd data) |

```yaml
data_path: ./data
```

## Nodes

Nodes represent the machines where loads are deployed. Each node has a name (the map key), a URL pointing to its daemon API, and a driver.

```yaml
nodes:
  lab1:
    url: http://192.168.1.59:9000
    driver: linux

  lab2:
    url: http://192.168.1.51:9000
    driver: linux
```

| Field           | Type   | Required | Description                                    |
| --------------- | ------ | -------- | ---------------------------------------------- |
| `url`           | string | Yes      | URL of the node's daemon API                   |
| `driver`        | string | Yes      | Node driver type. Currently supported: `linux` |
| `driver_config` | object | No       | Driver-specific configuration                  |
| `cloud_init`    | object | No       | Cloud init configuration                       |

**Cloud init **

Realm serves cloud-init configurations for any node with a `cloud_init` configuration. Currently Realm supports `meta-data`and `user-data` structures.

An usage example using the Qemu node driver:

```yaml
nodes:
  vm:
    url: http://localhost:9000
    cloud_init:
      meta_data:
        instance-id: test01
        local-hostname: testvm
      user_data:
        hostname: lab1
        fqdn: lab1.local
        preserve_hostname: false
        apt:
          sources:
            bookworm-backports:
              source: "deb http://deb.debian.org/debian bookworm-backports main contrib non-free-firmware"
        runcmd:
          - apt-get update
    driver: qemu
    driver_config:
      emulator: qemu-system-x86_64
      machine: q35
      accel: hvf
      memory: 2048
      smp: "2"
      serial: telnet:localhost:4444,server,nowait
      netdevs:
        - type: user
          id: net0
      drives:
        - file: /Users/juan/repos/vlab/debian-12-generic-amd64.qcow2
          format: qcow2
          if: virtio
      params:
        - -display
        - none
        - -device
        - virtio-net-pci,netdev=net0
```

### Linux Node Driver

The `linux` driver supports optional Wake-On-LAN:

```yaml
nodes:
  server:
    url: http://192.168.1.59:9000
    driver: linux
    driver_config:
      wol: true
      MAC: "00:11:22:33:44:55"
```

| Field | Type   | Description                               |
| ----- | ------ | ----------------------------------------- |
| `wol` | bool   | Enable Wake-On-LAN                        |
| `MAC` | string | MAC address (required when `wol` is true) |

## Loads

Loads are the workloads deployed to nodes. Each load has a name (the map key), a target node, a driver (`container` or `process`), and driver-specific configuration.

```yaml
loads:
  web:
    node: lab1
    driver: container
    driver_config:
      image: docker.io/nginx:latest

  my_service:
    node: lab2
    driver: process
    driver_config:
      start_cmd: node
      start_args: "server.js"
      stop_signal: SIGTERM
```

| Field           | Type   | Required | Description                                               |
| --------------- | ------ | -------- | --------------------------------------------------------- |
| `node`          | string | Yes      | Target node name or [expression](#dynamic-node-selection) |
| `driver`        | string | Yes      | Load driver: `container` or `process`                     |
| `driver_config` | object | Yes      | Driver-specific configuration                             |
| `depends_on`    | list   | No       | List of load names this load depends on                   |

For detailed driver configuration, see [Container Driver](container-driver.md) and [Process Driver](process-driver.md).

### Dependencies

Loads can declare dependencies on other loads using `depends_on`. Realm builds a dependency graph and ensures loads start and stop in the correct order. Circular dependencies are detected and rejected.

```yaml
loads:
  database:
    node: lab1
    driver: container
    driver_config:
      image: docker.io/postgres:15

  api:
    node: lab1
    driver: container
    driver_config:
      image: docker.io/myapi:latest
    depends_on:
      - database
```

In this example, `database` will always start before `api`, and `api` will stop before `database`.

### Dynamic Node Selection

The `node` field supports expressions using [expr-lang](https://expr-lang.org/) for dynamic node selection. The expression context provides:

- `nodes` - array of all configured nodes
- `selectAny(slice)` - randomly select one item from a slice

```yaml
nodes:
  vps-1:
    url: http://vps1:9000
    driver: linux
  vps-2:
    url: http://vps2:9000
    driver: linux

loads:
  web:
    node: selectAny(filter(nodes, .Name startsWith "vps-"))
    driver: container
    driver_config:
      image: docker.io/nginx:latest
```

## Daemon

The `daemon` section configures the Realm daemon. All fields are optional and have sensible defaults.

```yaml
daemon:
  listen_address: 0.0.0.0
  listen_port: 9000
  log_format: text
```

### General

| Field            | Type   | Default   | Description                         |
| ---------------- | ------ | --------- | ----------------------------------- |
| `listen_address` | string | `0.0.0.0` | Address to bind the daemon API      |
| `listen_port`    | int    | `9000`    | Port to bind the daemon API         |
| `log_format`     | string | `text`    | Log output format: `text` or `json` |

### Container Runtime

| Field                  | Type   | Default                           | Description                         |
| ---------------------- | ------ | --------------------------------- | ----------------------------------- |
| `containerd_sock`      | string | `/run/containerd/containerd.sock` | Containerd socket path              |
| `containerd_namespace` | string | `realm`                           | Containerd namespace                |
| `cni_path`             | string | `/usr/lib/cni`                    | Path to CNI plugins                 |
| `volumes_pool`         | string | `realm_volumes`                   | ZFS pool name for container volumes |
| `zfs`                  | bool   | `false`                           | Enable ZFS for volume management    |

### Reverse Proxy

| Field              | Type   | Default          | Description              |
| ------------------ | ------ | ---------------- | ------------------------ |
| `proxy_enabled`    | bool   | `false`          | Enable the reverse proxy |
| `local_caddy_url`  | string | `localhost:2019` | Local Caddy proxy URL    |
| `master_caddy_url` | string | `localhost:2019` | Master Caddy proxy URL   |

### Etcd

Realm uses etcd for cluster state. It can run an embedded etcd server or connect as a client to an external cluster.

| Field                    | Type   | Default                          | Description                                           |
| ------------------------ | ------ | -------------------------------- | ----------------------------------------------------- |
| `etcd_mode`              | string | `server`                         | Etcd mode: `server` (embedded) or `client` (external) |
| `etcd_endpoints`         | list   | `[]`                             | Etcd endpoints for client mode                        |
| `etcd_listen_client_url` | string | `http://<auto-detected-ip>:2379` | Etcd client URL                                       |
| `etcd_listen_peer_url`   | string | `http://<auto-detected-ip>:2380` | Etcd peer URL                                         |
| `etcd_initial_cluster`   | string | `""`                             | Initial cluster members (empty for single-node)       |

**Single-node (default):**

```yaml
daemon:
  listen_address: 0.0.0.0
```

**Multi-node server:**

```yaml
daemon:
  etcd_listen_client_url: http://192.168.1.10:2379
  etcd_listen_peer_url: http://192.168.1.10:2380
  listen_address: 0.0.0.0
```

**Client connecting to external etcd:**

```yaml
daemon:
  etcd_mode: client
  etcd_endpoints: ["http://192.168.1.10:2379"]
  etcd_listen_client_url: http://192.168.1.20:2379
  listen_address: 0.0.0.0
```

### Container Registries

Configure authentication for private container registries:

```yaml
daemon:
  registries:
    - host: ghcr.io
      auth:
        token: ghp_xxxxxxxxxxxx
    - host: registry.example.com:5000
      insecure: true
      auth:
        username: admin
        password: secret
```

| Field           | Type   | Description                                             |
| --------------- | ------ | ------------------------------------------------------- |
| `host`          | string | Registry host (e.g., `ghcr.io`, `docker.io`)            |
| `insecure`      | bool   | Allow HTTP instead of HTTPS                             |
| `auth.username` | string | Username (use with `password`)                          |
| `auth.password` | string | Password (use with `username`)                          |
| `auth.token`    | string | Authentication token (alternative to username/password) |

## Discovery

```yaml
discovery:
  mdns: true
```

| Field  | Type | Default | Description           |
| ------ | ---- | ------- | --------------------- |
| `mdns` | bool | `false` | Enable mDNS discovery |

## Environment Variables

All daemon configuration fields can be set via environment variables using the `REALM_` prefix. Nested fields use underscores as separators:

```bash
REALM_DAEMON_LISTEN_ADDRESS=0.0.0.0
REALM_DAEMON_LISTEN_PORT=9000
REALM_DATA_PATH=/opt/realm
REALM_DAEMON_ETCD_MODE=client
```

Environment variables take priority over config file values but are overridden by command-line flags.

## Complete Example

```yaml
data_path: /opt/realm_data
daemon:
  listen_address: 0.0.0.0
  zfs: false
  registries:
    - host: ghcr.io
      auth:
        token: ghp_xxxxxxxxxxxx

nodes:
  lab1:
    url: http://192.168.1.59:9000
    driver: linux
  lab2:
    url: http://192.168.1.51:9000
    driver: linux

loads:
  database:
    node: lab1
    driver: container
    driver_config:
      image: docker.io/postgres:15
      env:
        - POSTGRES_PASSWORD=secret
      network:
        network: backend
        dns: true
        ip_masq: true
      mount_volume:
        - volume_mount_point: /var/lib/postgresql/data

  api:
    node: lab1
    driver: container
    driver_config:
      image: ghcr.io/myorg/api:latest
      env:
        - DB_HOST=database.realm
      network:
        network: backend
        dns: true
        ip_masq: true
        port_map:
          - host_port: 8080
            container_port: 3000
            protocol: tcp
    depends_on:
      - database

  monitoring:
    node: lab2
    driver: process
    driver_config:
      start_cmd: prometheus
      start_args: "--config.file=/etc/prometheus/prometheus.yml"
      working_dir: /opt/prometheus
      stop_signal: SIGTERM

discovery:
  mdns: true
```

### QEMU Node Driver

TBD

### Windows Node Driver

TBD
