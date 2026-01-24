# QEMU load driver

## Configuration

### Basic Configuration

```
  vm1:
    node: lab1
    driver: qemu
    driver_config:
      image: /path/to/disk.qcow2
      cpus: 2
      memory: 2G
      machine_type: q35
      accelerator: kvm
      net_device: user
      net_options:
        id: net0
        hostfwd: tcp::2222-:22
      vnc_display: ":1"
      monitor_port: 4444
      serial_log: /tmp/vm1-serial.log
```

### Full Configuration Example

```yaml
loads:
  web_server_vm:
    node: lab1
    driver: qemu
    driver_config:
      # Required: Path to disk image
      image: /var/lib/vms/ubuntu-22.04.qcow2

      # CPU and Memory
      cpus: 4
      memory: 4G

      # Machine configuration
      machine_type: q35              # Default: pc
      accelerator: kvm               # Default: kvm (fallback to tcg if unavailable)
      qemu_binary: qemu-system-x86_64  # Default: qemu-system-x86_64

      # Network configuration
      net_device: user               # Options: user, bridge, tap
      net_options:
        id: net0
        hostfwd: tcp::8080-:80       # Forward host port 8080 to VM port 80
        hostfwd: tcp::2222-:22       # Forward host port 2222 to VM port 22

      # Display options
      vnc_display: ":1"              # VNC on display :1 (port 5901)
      no_graphic: false              # Set to true to disable graphics

      # Monitoring and logging
      serial_log: /var/log/realm/vm-serial.log
      monitor_port: 4444             # QMP monitor on TCP port 4444

      # Working directory
      working_dir: /var/lib/vms

      # Additional QEMU arguments
      extra_args:
        - "-boot"
        - "order=c"
```

## Configuration Reference

### Required Fields

| Field | Type | Description |
|-------|------|-------------|
| `image` | string | Path to the VM disk image (qcow2, raw, etc.) |

### Optional Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `cpus` | int | 1 | Number of virtual CPUs |
| `memory` | string | "1G" | Memory size (e.g., "512M", "2G", "4096M") |
| `machine_type` | string | "pc" | QEMU machine type (pc, q35, etc.) |
| `accelerator` | string | "kvm" | Acceleration type (kvm, tcg, hvf) |
| `qemu_binary` | string | "qemu-system-x86_64" | Path to QEMU binary |
| `net_device` | string | "user" | Network backend type |
| `net_options` | map | {} | Additional network options |
| `vnc_display` | string | "" | VNC display (e.g., ":0", ":1") |
| `no_graphic` | bool | false | Disable graphical output |
| `serial_log` | string | "" | Path to serial console log file |
| `monitor_port` | int | 0 | TCP port for QMP monitor |
| `working_dir` | string | (image dir) | Working directory for QEMU |
| `additional_disks` | []DiskImage | [] | Additional disk images to attach |
| `usb_devices` | []USBDevice | [] | USB devices to pass through |
| `cloud_init` | CloudInitConfig | nil | Cloud-init configuration |
| `boot` | BootConfig | nil | Boot configuration options |
| `snapshot` | bool | false | Run in snapshot mode (changes not saved) |
| `cpu_flags` | []string | [] | Additional CPU flags (e.g., "+vmx", "+svm") |
| `numa` | bool | false | Enable NUMA configuration |
| `hugepages` | bool | false | Use hugepages for memory |
| `rtc` | string | "base=utc,clock=host" | RTC configuration |
| `extra_args` | []string | [] | Additional QEMU command-line arguments |
