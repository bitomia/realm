# Configuration

Realm is configured through a YAML file. By default, Realm looks for a `config.yaml` file in the same directory as the executable. You can specify a custom path using the `--config` flag or the `REALM_CONFIG_FILE` environment variable.

## File Structure

A Realm configuration file has four top-level sections:

```yaml
agent: # Agent settings (paths, etcd, registries)
nodes: # Remote nodes to manage
loads: # Workloads to deploy
discovery: # Discovery settings
```

## Root config

| Field       | Type   | Default          | Description                                              |
| ----------- | ------ | ---------------- | -------------------------------------------------------- |
| `data_path` | string | `/var/lib/realm` | Path to store client or agent data (ID file, etcd data) |

```yaml
data_path: ./data
```

## Nodes

Nodes represent the machines where loads are deployed. Each node has a name (the map key), a URL pointing to its agent API, and a driver.

```yaml
nodes:
  lab1:
    url: http://192.168.1.59:9000
    driver: linux

  lab2:
    url: http://192.168.1.51:9000
    driver: linux
```

`url` field can be also configured with a mDNS address:

```yaml
nodes:
  lab1:
    url: http://lab1.local:9000
...
```


| Field           | Type   | Required | Description                                    |
| --------------- | ------ | -------- | ---------------------------------------------- |
| `url`           | string | Yes      | URL of the node's agent API                   |
| `driver`        | string | Yes      | Node driver type. Currently supported: `linux` |
| `driver_config` | object | No       | Driver-specific configuration                  |
| `cloud_init`    | object | No       | Cloud init configuration                       |

**Cloud init**

Realm serves cloud-init configurations for any node with a `cloud_init` configuration. Currently Realm supports `meta-data`and `user-data` structures.

An usage example using the VM node driver:

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
    driver: vm
    driver_config:
      machine: q35
      accel: hvf
      memory: 2048
      smp: "2"
      serial: telnet:localhost:4444,server,nowait
      netdevs:
        - type: user
          id: net0
      drives:
        - file: /home/user/debian-12-generic-amd64.qcow2
          format: qcow2
          if: virtio
```

### Linux Node Driver

The `linux` driver runs on a host where Realm is installed as a agent. It supports optional Wake-On-LAN to power the host on remotely:

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

### Windows Node Driver

```yaml
nodes:
  workstation:
    url: http://192.168.1.20:9000
    driver: windows
    driver_config:
      wol: true
      MAC: "00:11:22:33:44:55"
```

| Field | Type   | Description                               |
| ----- | ------ | ----------------------------------------- |
| `wol` | bool   | Enable Wake-On-LAN                        |
| `MAC` | string | MAC address (required when `wol` is true) |

### VM Node Driver

> **Experimental — Work in Progress**
>
> The VM driver is experimental and under active development. Its configuration schema, behavior, and defaults may change without notice, and it is not yet recommended for production use. Expect rough edges and please report issues you encounter.

The `vm` driver provisions guest nodes through a local **libvirtd** agent.

User running Realm must be allowed to run libvirt, set in `/etc/libvirt/qemu.conf`:

```
user = "<realm-user>"
group = "<realm-user>"
```

Now, restart libvirtd with `sudo systemctl restart libvirtd` so the overlay images under `<data_path>/overlays/` are readable by the QEMU process.

```yaml
nodes:
  vm:
    url: http://localhost:9000
    driver: vm
    driver_config:
      machine: q35
      accel: kvm
      cpu: host
      memory: 2048
      smp: "2"
      serial: telnet:localhost:4444,server,nowait
      drives:
        - file: https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-generic-amd64.qcow2
          format: qcow2
          if: virtio
      netdevs:
        - type: user
          id: net0
```

#### VMConfig Fields

The fields below describe a VM in driver-neutral terms; the driver translates them into a libvirt domain XML document. The `emulator` path is recorded as the `<emulator>` element, so libvirt invokes the binary you specify.

| Field     | Type           | Required | Description                                                                                                                      |
| --------- | -------------- | -------- | -------------------------------------------------------------------------------------------------------------------------------- |
| `machine` | string         | No       | Machine type (e.g. `q35`, `pc`). Mapped to `<os><type machine=...>`.                                                             |
| `accel`   | string or list | No       | Accelerator selection. The first recognised entry (`kvm`, `hvf`, `xen`) chooses the libvirt domain type; otherwise `qemu` (TCG). |
| `cpu`     | string         | No       | CPU model (e.g. `host`). Mapped to `<cpu mode='custom'><model>...</model></cpu>`.                                                |
| `memory`  | int            | No       | Memory in MiB. Mapped to `<memory unit='MiB'>`.                                                                                  |
| `smp`     | string         | No       | vCPU count. Either an integer (`"2"`) or a `cpus=N,...` form; the leading integer is used.                                       |
| `serial`  | string         | No       | Serial backend. `stdio` / `pty` produce a `pty` device; `file:/path` redirects to a file; `none` disables it.                    |
| `drives`  | list           | No       | Drive definitions, see below.                                                                                                    |
| `netdevs` | list           | No       | Netdev definitions, see below.                                                                                                   |

#### Drive Fields

| Field    | Type   | Description                                                                                                                                           |
| -------- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| `file`   | string | Path to the disk image. If it begins with `http://` or `https://` the image is downloaded and cached under `<data_path>/images/` (keyed by URL hash). |
| `format` | string | Image format (e.g. `qcow2`, `raw`).                                                                                                                   |
| `if`     | string | Drive interface (e.g. `virtio`, `ide`).                                                                                                               |
| `media`  | string | Media type (e.g. `disk`, `cdrom`).                                                                                                                    |
| `index`  | string | Drive index.                                                                                                                                          |

For each drive Realm copies the source image into a per-node overlay under `<data_path>/overlays/<node>/<uuid>` and points the libvirt `<disk>` at the copy, so the original image is never written to. Overlays are removed on deprovision.

#### Netdev Fields

| Field        | Type   | Description                                                                |
| ------------ | ------ | -------------------------------------------------------------------------- |
| `type`       | string | Netdev type (e.g. `user`, `tap`, `bridge`).                                |
| `id`         | string | Netdev id (referenced by `-device ...,netdev=<id>`).                       |
| `ifname`     | string | TAP interface name.                                                        |
| `script`     | string | TAP up script.                                                             |
| `downscript` | string | TAP down script.                                                           |
| `br`         | string | Bridge name (used by `bridge` type and for cloud-init host IP resolution). |
| `helper`     | string | Bridge helper binary.                                                      |
| `net`        | string | User-mode network range.                                                   |
| `dhcpstart`  | string | First DHCP address for user-mode networking.                               |
| `hostfwd`    | string | User-mode host port forwarding rule (e.g. `tcp::2222-:22`).                |

#### Cloud-init

When the node has a `cloud_init` block, the guest's cloud-init datasource fetches metadata from the Realm agent. The `<host>` is resolved as follows:

1. If any non-`user` netdev is configured and its `br` interface has an IPv4 address, that address is used.
2. Otherwise, the agent's auto-detected IP is used.
3. Otherwise, the configured `agent.listen_address` (if not `0.0.0.0`) is used.
4. Fallback: `10.0.2.2` (the QEMU user-mode gateway), which only works when the guest uses user-mode networking.

#### Logs

Per-VM logs are managed by libvirtd (typically under `/var/log/libvirt/qemu/<node>.log`). Inspect domain state with `virsh list --all` and `virsh dominfo <node>`.

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

## Agent

The `agent` section configures the Realm agent. All fields are optional and have sensible defaults.

```yaml
agent:
  listen_address: 0.0.0.0
  listen_port: 9000
  log_format: text
```

### General

| Field            | Type   | Default   | Description                         |
| ---------------- | ------ | --------- | ----------------------------------- |
| `listen_address` | string | `0.0.0.0` | Address to bind the agent API      |
| `listen_port`    | int    | `9000`    | Port to bind the agent API         |
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
agent:
  listen_address: 0.0.0.0
```

**Multi-node server:**

```yaml
agent:
  etcd_listen_client_url: http://192.168.1.10:2379
  etcd_listen_peer_url: http://192.168.1.10:2380
  listen_address: 0.0.0.0
```

**Client connecting to external etcd:**

```yaml
agent:
  etcd_mode: client
  etcd_endpoints: ["http://192.168.1.10:2379"]
  etcd_listen_client_url: http://192.168.1.20:2379
  listen_address: 0.0.0.0
```

### Container Registries

Configure authentication for private container registries:

```yaml
agent:
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

All agent configuration fields can be set via environment variables using the `REALM_` prefix. Nested fields use underscores as separators:

```bash
REALM_AGENT_LISTEN_ADDRESS=0.0.0.0
REALM_AGENT_LISTEN_PORT=9000
REALM_DATA_PATH=/opt/realm
REALM_AGENT_ETCD_MODE=client
```

Environment variables take priority over config file values but are overridden by command-line flags.

## Complete Example

```yaml
data_path: /opt/realm_data
agent:
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
