# Development setup

Recommended development setups are Debian 12 or Windows 11 Pro with Go >=1.24 installed.

## Debian 12 setup

Install ansible:

```shell
apt update
apt install ansible -y
```

### Building with ZFS support (optional)

ZFS volume support is optional and enabled at build time with the `zfs` build tag. To build with ZFS, you will need to install the ZFS development libraries from [Debian Bookworm Backports](https://backports.debian.org/Instructions/).

Install backports as follows:

```shell
cat > /etc/apt/sources.list.d/bookworm-backports.list << EOF
deb http://deb.debian.org/debian bookworm-backports main contrib non-free-firmware
EOF
```

Now install the ZFS development dependencies:

```shell
apt update
apt install zfsutils-linux libzfslinux-dev -y
```

Then build with:

```shell
make TAGS=zfs
```

Before starting the daemon with ZFS support, create the ZFS pool:

```shell
sudo zpool create realm_volumes /dev/sdX  # Replace /dev/sdX with your device
```

Without the `zfs` tag, Realm uses directory-based volumes which work on all platforms with no extra dependencies.

## Windows 11 Pro setup

We recommend to use only Powershell and check that you don't use msys2 or have another unix shell installed, **make** can conflicts with these shells.

Install building dependencies (required for CGO):

```powershell
choco install mingw
```

Install golang: https://go.dev/doc/install

Install containerd:

```powershell
Enable-WindowsOptionalFeature -Online -FeatureName containers -All

mkdir "c:\Program Files\containerd"
cd "c:\Program Files\containerd"

curl -L https://github.com/containerd/containerd/releases/download/v2.2.1/containerd-2.2.1-windows-amd64.tar.gz -o containerd-windows-amd64.tar.gz
tar xvf .\containerd-windows-amd64.tar.gz -C "c:\Program Files\containerd"

$Path = [Environment]::GetEnvironmentVariable("PATH", "Machine") + [IO.Path]::PathSeparator + "$Env:ProgramFiles\containerd\bin"
[Environment]::SetEnvironmentVariable("Path", $Path, "Machine")

containerd config default | Out-File "c:\Program Files\containerd\config.toml" -Encoding ascii
containerd --register-service

net start containerd
```
