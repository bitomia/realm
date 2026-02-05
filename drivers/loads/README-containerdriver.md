# Container Driver

## Volume Configuration

The container driver supports two types of volume mounts:

1. **Managed Volumes** (`mount_volume`) - System-created volumes with optional size quotas
2. **Bind Mounts** (`bind_mounts`) - Direct host path mounts (similar to Docker's `-v` flag)

### Bind Mounts

Bind mounts allow you to mount existing host directories directly into the container:

```yaml
loads:
  my_app:
    node: lab1
    driver: container
    driver_config:
      image: docker.io/myapp:latest
      bind_mounts:
        - source: ./data
          destination: /app/data
        - source: /opt/config
          destination: /opt/config
          readonly: true
        - source: /var/log/myapp
          destination: /var/log/app
```

#### Bind Mount Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `source` | string | Yes | Path on the host machine (absolute or relative) |
| `destination` | string | Yes | Path inside the container |
| `readonly` | bool | No | Mount as read-only. Default: false |

### Managed Volumes

Managed volumes are created and managed by the system (ZFS-backed or directory-based). They support size quotas (only ZFS) and are **not automatically cleaned up** when containers are deleted:

```yaml
loads:
  database:
    node: lab1
    driver: container
    driver_config:
      image: docker.io/postgres:15
      mount_volume:
        - volume_mount_point: /var/lib/postgresql/data
          volume_size: 10G
```

#### Managed Volume Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `volume_mount_point` | string | Yes | Path inside the container where the volume is mounted |
| `volume_size` | string | No | Size quota for the volume (e.g., "10G", "500M"). Only enforced with ZFS volumes. |

**Note on `volume_size`:**
- **With ZFS volumes** (built with `make TAGS=zfs`): Quota is enforced at the ZFS dataset level
- **With directory volumes** (default build): Quota is ignored with a warning logged. The volume is still created and mounted, but without size restrictions

To use ZFS quotas, ensure:
1. Realm is built with ZFS support: `make TAGS=zfs`
2. ZFS pool is configured: `daemon.zfs: true` in config
3. ZFS pool exists and is accessible

### Example

```yaml
loads:
  web_app:
    node: lab1
    driver: container
    driver_config:
      image: docker.io/nginx:latest
      # Bind mount for configuration (read-only)
      bind_mounts:
        - source: /opt/test/nginx.conf
          destination: /etc/nginx/nginx.conf
          readonly: true
        - source: /opt/test/html
          destination: /usr/share/nginx/html
          readonly: true
      # Managed volume for logs with quota
      mount_volume:
        - volume_mount_point: /var/log/nginx
          volume_size: 1G
```

## Network Configuration

The container load driver supports automatic network configuration using CNI (Container Network Interface).

## Features

- Automatic network attachment when containers start
- CNI-based networking with bridge, firewall, and portmap plugins
- DNS registration for container discovery (`.realm.` domain)
- Port mapping from host to container
- IP masquerading support
- Automatic network cleanup when containers stop

## Configuration

### Basic Network Configuration

Add a `network` field to your container driver configuration:

```yaml
loads:
  web_app:
    node: lab1
    driver: container
    driver_config:
      image: docker.io/nginx:latest
      network:
        network: my-network    # Network name
        ip_masq: true          # Enable IP masquerading (NAT)
        dns: true              # Enable DNS registration
```

### Network with Port Mapping

Map ports from the host to the container:

```yaml
loads:
  api_server:
    node: lab1
    driver: container
    driver_config:
      image: docker.io/myapp:latest
      network:
        network: app-network
        ip_masq: true
        dns: true
        port_map:
          - host_port: 8080
            container_port: 80
            protocol: tcp
          - host_port: 8443
            container_port: 443
            protocol: tcp
```

### Multiple Containers on Same Network

Containers on the same network can communicate with each other:

```yaml
loads:
  database:
    node: lab1
    driver: container
    driver_config:
      image: docker.io/postgres:15
      env:
        - POSTGRES_PASSWORD=secret
      network:
        network: backend-network
        dns: true

  api:
    node: lab1
    driver: container
    driver_config:
      image: docker.io/myapi:latest
      env:
        - DB_HOST=database.realm  # Use DNS name
      network:
        network: backend-network
        dns: true
        port_map:
          - host_port: 3000
            container_port: 3000
            protocol: tcp
    depends_on:
      - database
```

## Network Configuration Reference

### StartNetworkRequest Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `network` | string | Yes | Network name. Containers on the same network can communicate. |
| `ip_masq` | bool | No | Enable IP masquerading (NAT) for outbound traffic. Default: false |
| `dns` | bool | No | Register container in DNS for `.realm.` domain resolution. Default: false |
| `port_map` | []Portmap | No | Port mappings from host to container |

### Portmap Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `host_port` | uint16 | Yes | Port on the host machine |
| `container_port` | uint16 | Yes | Port inside the container |
| `protocol` | string | Yes | Protocol: "tcp" or "udp" |

### DNS Resolution

When DNS is enabled:

- Containers are registered as `<container-name>.realm.`
- DNS server runs on port 15353
- Automatically added to container's `/etc/resolv.conf`
- Other containers on the same network can resolve by name

## Subnet Allocation

- Networks are assigned /24 subnets from the 10.0.0.0/8 range
- Subnet allocation is persistent across daemon restarts
- Each network name gets a consistent subnet
- Gateway is always `.1` in the subnet

## Troubleshooting

### Container can't reach the internet

Ensure `ip_masq: true` is set in the network configuration:

```yaml
network:
  network: my-network
  ip_masq: true  # Required for internet access
```

### Port mapping not working

1. Check that the port is not already in use on the host
2. Verify the protocol matches (tcp vs udp)
3. Check firewall rules on the host

### DNS resolution fails

1. Ensure `dns: true` is set in the network configuration
2. Check that containers are on the same network
3. Verify DNS server is running (port 15353)

### Network cleanup issues

If networks aren't being cleaned up properly, you can manually purge orphaned networks:

```bash
# Using the daemon API
curl -X POST http://daemon:9000/network
```
