# Getting started

## Installation

Realm running as client does not require any external dependencies. The following guidelines are only for Realm daemon.

### Linux

```
sudo apt update
sudo apt install -y containerd containernetworking-plugins zfsutils-linux zfs-dkms
```

### Windows

Realm daemon can deploy native windows containers. Follow these steps to enable them on your Windows system:

1. Enable native Windows containers:

```powershell
Enable-WindowsOptionalFeature -Online -FeatureName Containers -All
```

2. Download Windows containerd binaries from the [official repository](https://github.com/containerd/containerd/releases)
3. Extract to `C:\Program Files\containerd`
4. Generate config:

```powershell
containerd config default > C:\Program Files\containerd\config.toml
```

5. Register as a Windows service:

```powershell
containerd.exe --register-service
Start-Service containerd
```

6. Verify it is running:

```powershell
Get-Service containerd
C:\Program Files\containerd\bin\ctr version
```

Now you will need to install CNI plugins to enable container networking:

1. Download Windows CNI binaries from the [official repository](https://github.com/containernetworking/plugins/releases)
2. Extract to `C:\Program Files\containerd\cni\bin`

Note: all the recommended installation paths are also the default paths used by Realm, but you can also set different paths on the Realm configuration file.
