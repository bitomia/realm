# Container Driver

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
