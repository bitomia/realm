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

## driver_config reference

| Field            | Type           | Default                         | Description                                                       |
| ---------------- | -------------- | ------------------------------- | ----------------------------------------------------------------- |
| `machine`        | string         | —                               | QEMU machine type (e.g. `q35`)                                    |
| `accel`          | string or list | —                               | Acceleration backends tried in order (e.g. `kvm`, `hvf`)          |
| `cpu`            | string         | —                               | CPU model, or `host` / `host-model` to name a mode                |
| `memory`         | int            | —                               | RAM in MiB                                                        |
| `smp`            | string         | —                               | SMP topology                                                      |
| `serial`         | string         | —                               | Serial device string (e.g. `telnet:localhost:4444,server,nowait`) |
| `bios`           | object         | —                               | UEFI firmware. See [Firmware](#firmware)                          |
| `tpm`            | object         | —                               | Emulated TPM. See [TPM](#tpm)                                     |
| `graphics`       | object         | —                               | Remote display and video device. See [Graphics](#graphics)        |
| `drives`         | list           | —                               | Disk images to attach                                             |
| `netdev`         | list           | —                               | Network devices to attach                                         |
| `params`         | list           | —                               | Extra QEMU arguments                                              |
| `libvirt_socket` | string         | `/var/run/libvirt/libvirt-sock` | Path to the libvirt UNIX socket                                   |

### Firmware

Without a `bios` block the guest boots on the QEMU default firmware (SeaBIOS). Setting `bios.loader` switches it to UEFI, attached as pflash.

| Field            | Type   | Default | Description                                                        |
| ---------------- | ------ | ------- | ------------------------------------------------------------------ |
| `loader`         | string | —       | Path to the read-only firmware code image                          |
| `nvram`          | string | —       | Path to this node's writable variable store; required with `loader`|
| `nvram_template` | string | —       | Image the variable store is instantiated from when absent          |
| `secure`         | bool   | `false` | Enable Secure Boot                                                 |

```yaml
driver_config:
  machine: q35
  bios:
    loader: /usr/share/OVMF/OVMF_CODE_4M.secboot.fd
    nvram: ./data/win1_VARS.fd
    nvram_template: /usr/share/OVMF/OVMF_VARS_4M.ms.fd
    secure: true
```

### TPM

| Field     | Type   | Default   | Description                     |
| --------- | ------ | --------- | ------------------------------- |
| `model`   | string | `tpm-crb` | Device model (`tpm-crb`, `tpm-tis`) |
| `version` | string | `2.0`     | TPM version (`2.0`, `1.2`)      |

The backend is `swtpm`, which libvirt starts and supervises per domain. Windows 11 requires a TPM 2.0 alongside UEFI Secure Boot.

```yaml
driver_config:
  tpm:
    model: tpm-crb
    version: "2.0"
```

### Graphics

A `graphics` block attaches both the remote display and the video device it draws on.

| Field    | Type   | Default     | Description                            |
| -------- | ------ | ----------- | -------------------------------------- |
| `type`   | string | `vnc`       | `vnc` or `spice`                       |
| `listen` | string | `127.0.0.1` | Listen address                         |
| `port`   | int    | autoport    | Fixed port; omit to let libvirt pick   |
| `video`  | string | `virtio`    | Video device model                     |

The default listen address is loopback. Note that a display on all interfaces is unauthenticated access to the guest console.

```yaml
driver_config:
  graphics:
    type: vnc
    listen: 127.0.0.1
    video: virtio
```

## Cloud-init

When a node has a `cloud_init` block, the driver configures the guest to fetch it
via the NoCloud datasource over HTTP.

The `cloud_init_host` variable will be exposed to the `cloud_init` block and it will be
automatically expanded when `{{cloud_init_host}}` is found.

## Troubleshooting

### VM on a bridge never gets an IP

The guest hangs on `systemd-networkd-wait-online`, no lease appears in
`virsh net-dhcp-leases default`, and cloud-init never runs because it has no
network to fetch its datasource over.

First check that the network is up at all with `virsh -c qemu:///system net-list`
should show `default` active, and `ip -br link show master virbr0` should list
the guest's tap as `UP`. If both look right, the DHCP request is reaching the
host and being dropped there.

With firewalld, libvirt normally assigns `virbr0` to the `libvirt` zone, which
permits `dhcp`, `dns`, `ssh` and `tftp`. When that assignment does not happen the
bridge falls through to the default zone, which drops DHCP:

```bash
firewall-cmd --get-zone-of-interface=virbr0   # "no zone" usually indicates an issue
firewall-cmd --zone=libvirt --change-interface=virbr0
firewall-cmd --runtime-to-permanent
```

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

### Guest cannot open a file passed through `params`

QEMU exits at startup with `Could not open '<path>': Permission denied`, on a
path that is world-readable and that the agent itself can read.

On hosts running AppArmor or SELinux, the guest runs under a per-domain profile
built by `virt-aa-helper`, which grants access only to paths it can see in the
domain XML. Anything reached through `qemu:commandline` is invisible to it and
is denied at `open()`, no matter what the file permissions say.

Use the driver field that puts the path in the XML — `bios` for pflash images,
`tpm` for the TPM state, `drives` for disks — rather than passing it as a raw
QEMU argument.

### Domain fails with "Unknown CPU model host"

`cpu: host` and `cpu: host-model` name libvirt CPU *modes*, not models, and the
driver maps them accordingly. Any other value is passed through as
`mode='custom'`, so it has to be a model libvirt knows; list the ones this host
can run with:

```bash
virsh -c qemu:///system domcapabilities | grep "usable='yes'"
```
