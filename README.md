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

## Production setup (deprecated)

### How to configure a production node (deprecated)

These are old instructions to configure a production node. You might want to check **dev** folder if you are configuring your local development environment instead.

Realm is a simple orchestration service for OCI containers based on containerd. It will run as a cluster with different nodes running *realm daemon* instances (check 'daemon' folder) and managing the cluster should be commanded with the CLI (check 'cli' folder) or using the REST API that each daemon exposes.

In the near future we want to have a web application to command it as well.

### How to configure a production node (deprecated)

These are old instructions to configure a production node. You might want to check **dev-setup** folder if you are configuring your local development environment instead.

```
# Verify Caddy configuration
# caddy.service should be disabled and caddy-api.service should enabled instead

systemctl stop caddy
systemctl disable caddy
systemctl status caddy

systemctl enable caddy-api
systemctl start caddy-api
systemctl status caddy-api
```

caddy-api uses "--resume" option.

If you are in production, you will need to open admin API in the master caddy to the rest of realmd hosts as follow:

```
# SSH to master caddy

# Filter port to realmd
iptables -A INPUT -p tcp -s <REALMD_HOST_IP> --dport 2019 -j ACCEPT
iptables -A INPUT -p tcp -s 127.0.0.1 --dport 2019 -j ACCEPT

# Drop rest of connections
iptables -A INPUT -p tcp --dport 2019 -j DROP
iptables -A INPUT -p udp --dport 2019 -j DROP

# Set basic caddy "master" server config
curl -X POST http://localhost:2019/config --data '{"admin":{"listen":":2019"},"apps":{"http":{"servers":{"master":{"listen":[":80",":443"],"routes":[],"tls_connection_policies":[{}]}}}},"logging":{"logs":{"default":{"encoder":{"format":"json"},"level":"INFO","writer":{"filename":"/var/log/caddy/access.log","output":"file"}}}}}' -H "Content-Type: application/json"
```

#### Restarting Caddy ####

Save first current config just in case auto save didn't work:
```
curl "http://localhost:2019/config/apps/http/servers" | jq > backup.json
```

Now restart:
```
systemctl restart caddy-api
```

WHAT WE HAVE TESTED is restoting the servers config not the admin config, so the admin listen to all interfaces of the 2019 API is not preserved. Do this to restore and check it is not accesible from unauthorized hosts. IN FUTURE TESTS try to save the backup for everything:

```
curl -X POST http://localhost:2019/config/admin --data '{ "listen": ":2019" }' -H "Content-Type: application/json"
```

# Install containerd
sudo apt install containerd -y

# Install CNI tool
git clone https://github.com/containernetworking/cni.git
cd cni/cnitool
go build .
sudo cp cnitool /usr/local/bin/

# Install CNI plugins
git clone https://github.com/containernetworking/plugins.git
cd plugins
./build_linux.sh

sudo mkdir -p /opt/cni/bin
sudo cp bin/* /opt/cni/bin
sudo mkdir -p /etc/cni/net.d

# Install caddy
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
sudo apt update
sudo apt install caddy
systemctl status caddy
```

### Volumes

```
# Install deps
sudo apt install zfsutils-linux libzfslinux-dev -y

# Check block device
lsblk

# Create volumes pool
sudo zpool create realmpool /dev/sdb

# Check zfs pool status
zpool status
```

### DNS

realmd binds a DNS server to port 15353 by default. Follow the next instructions to configure your host:

Install dnsmasq and check that's it configured in resolv.conf, for example the following are the entries you should have for a host using dnsmasq and systemd-resolv:

```
nameserver 127.0.0.1
nameserver 127.0.0.53
options edns0 trust-ad
nameserver 8.8.8.8
nameserver 1.1.1.1
search .
```

Note: Have in mind that /etc/resolv.conf is symbolic-linked by systemd-resolv, you need to remove the symbolic link.

Now configure dnsmasq to use the realm DNS server as the upstream server for all the ".realm" domains. In order to do that, add the following line to /etc/dnsmasq.conf:

```
listen-address=0.0.0.0
...
server=/realm/127.0.0.1#15353
```

systemd-resolved can conflict with dnsmasq, so the following systemd-resolved conf is important:

```
DNSStubListener=no
```

systemd-resolved can conflict with dnsmasq, so the following restart order is important:

```
systemctl stop systemd-resolved
systemctl restart dnsmasq
systemctl restart systemd-resolved
```

Now dnsmasq should be listening on 0.0.0.0:53, container names can be resolved and also internet URLs.

NOTE: YOU CAN VERIFY WITH PING TO A CONTAINER IF IT WORKS. E.G.: `ping wp_XXXXX.realm`
NOTE2: VERIFY THAT THE SYMBOLIC LINK IN /etc/resolv.conf DOESN'T EXIST ANYMORE


### Security

All containers shall not have any capabilities. For example we don't set NET_ADMIN (https://man7.org/linux/man-pages/man7/capabilities.7.html) to prevent containers modifying routing tables what could allow them to have access to other containers outside of its internal network.

