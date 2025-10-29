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

