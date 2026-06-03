# VM Driver

The VM driver provisions and manages virtual machines as guest nodes through [libvirt](https://libvirt.org/).

## Configuration

### Example

```yaml
url: http://localhost:9000
driver: vm
driver_config:
  machine: q35
  memory: 2048
  accel:
    - kvm
    - hvf
  smp: "2"
  serial: telnet:localhost:4444,server,nowait
  netdev:
    - type: bridge
      br: virbr0
      mac: 52:54:00:12:34:57
  drives:
    - file: https://cloud.debian.org/images/cloud/bookworm/latest/debian-12-generic-amd64.qcow2
      format: qcow2
      if: virtio
      resize: 80G
  params:
    - -display
    - vga
cloud_init:
  meta_data:
    instance-id: sen1
    local-hostname: sen1
  user_data:
    hostname: sen1
    users:
      - name: test
        shell: /bin/bash
        sudo: ALL=(ALL) NOPASSWD:ALL
        ssh_authorized_keys: ssh-ed25519 AAAA... test@local
    ssh_pwauth: true
    packages:
      - podman
```

## Cloud-init

When a node has a `cloud_init` block, the driver configures the guest to fetch it
via the NoCloud datasource over HTTP.

## Troubleshooting

### VM gets an IP but cloud-init never applies

The agent binds to `agent.listen_address` (default `0.0.0.0`). If it is set to a
specific IP or `127.0.0.1`, guests on the bridge cannot reach it. Use `0.0.0.0`
or the bridge-facing address.

Another common cause is a **host firewall blocking the agent on the
bridge**. With bridged networking, the guest reaches the agent at the bridge IP
(e.g. `http://192.168.122.1:9000/cloudinit/<node>/`). On hosts running firewalld,
libvirt places `virbr0` in the `libvirt` zone, which only permits `dhcp`, `dns`,
`ssh`, and `tftp`.

A possible fix is to open the agent port to the bridge, scoped to the libvirt zone/interface
(replace `9000` with your configured `agent.listen_port`):

```bash
# firewalld
firewall-cmd --permanent --zone=libvirt --add-port=9000/tcp
firewall-cmd --reload

# nftables
nft insert rule inet filter input iifname "virbr*" tcp dport 9000 accept

# iptables (not persistent on its own)
iptables -I INPUT -i virbr0 -p tcp --dport 9000 -j ACCEPT
```
