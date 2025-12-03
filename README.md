# Realm - Simple container orchestration service

Realm is a extendable, embeddable and simple orchestration service for different type of loads such as native processes or OCI containers.

It's simple because it is just one executable to command the cluster where Realm runs as daemon on each one of the cluster nodes. It's also extendable because it uses a driver systems to extend it with custom loads or node drivers. Managing the cluster can be commanded from command-line interface or using the REST API that each daemon exposes. It's embeddable because Realm provides a C API to interface with clusters.

## Development setup

Recommended setup is Debian 12 or Windows 11 Pro with Go >=1.24 installed. 

### Windows 11 Pro setup

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
curl.exe -L https://github.com/containerd/containerd/releases/download/v2.2.1/containerd-2.2.1-windows-amd64.tar.gz -o containerd-windows-amd64.tar.gz
tar.exe xvf .\containerd-windows-amd64.tar.gz -C "c:\Program Files\containerd"
$Path = [Environment]::GetEnvironmentVariable("PATH", "Machine") + [IO.Path]::PathSeparator + "$Env:ProgramFiles\containerd\bin"
[Environment]::SetEnvironmentVariable("Path", $Path, "Machine")
containerd.exe config default | Out-File "c:\Program Files\containerd\config.toml" -Encoding ascii
containerd --register-service
net start containerd
```

### Debian 12 setup

To build realm you will need also to install some ZFS dependencies from [Debian Bookworm Backports](https://backports.debian.org/Instructions/).

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

## Configuration

Realm can be configured using a YAML configuration file (`realm.yaml`) or environment variables.

### Daemon Configuration

The daemon section configures the realm daemon behavior. All fields are optional and have default values:

```yaml
daemon:
  id_path: /var/lib/realm/realm.id          # Path to store daemon unique ID (default: platform-specific)
  cni_path: /opt/cni                        # Path to CNI plugins (default: /opt/cni on Linux)
  volumes_pool: realm_volumes               # Name of ZFS pool for volumes (default: realm_volumes)
  listen_address: 127.0.0.1                 # Address to bind the daemon API (default: 127.0.0.1)
  listen_port: 9000                         # Port to bind the daemon API (default: 9000)
  logs_path: /var/log/realm                 # Path to store daemon logs (default: /var/log/realm on Linux)
  log_format: text                          # Log format: "text" or "json" (default: text)
  containers_log_path: /var/log/realm/containers  # Path to store container logs (default: platform-specific)
  proxy_enabled: false                      # Enable/disable the proxy (default: false)
  local_caddy_url: localhost:2019           # Local Caddy admin URL (default: localhost:2019)
  master_caddy_url: localhost:2019          # Master Caddy admin URL (default: localhost:2019)
  github_registry_token: ""                 # Token for GitHub registry (default: empty)
  containerd_sock: /run/containerd/containerd.sock  # Containerd socket path (default: platform-specific)
  containerd_namespace: realm               # Containerd namespace (default: realm)
  etcd_data_dir: /var/lib/realm/etcd        # etcd data directory (default: /var/lib/realm/etcd on Linux)
  etcd_name: ""                             # etcd member name (default: empty)
  etcd_listen_client_url: http://127.0.0.1:2379   # etcd client URL (default: http://127.0.0.1:2379)
  etcd_listen_peer_url: http://127.0.0.1:2380     # etcd peer URL (default: http://127.0.0.1:2380)
  etcd_initial_cluster: ""                  # etcd initial cluster configuration (default: empty)
  etcd_cluster_state: new                   # etcd cluster state: "new" or "existing" (default: new)
```

#### Environment Variables

Configuration values can be overridden using environment variables with the `REALM_` prefix and underscores for nested values:

```bash
REALM_DAEMON_LOG_FORMAT=json
REALM_DAEMON_LISTEN_PORT=9001
```

### Security

All containers shall not have any capabilities. For example we don't set NET_ADMIN (https://man7.org/linux/man-pages/man7/capabilities.7.html) to prevent containers modifying routing tables what could allow them to have access to other containers outside of its internal network.

## Contributing

Please follow the guidelines below to ensure code quality and consistency.

### Code Style and Conventions

This project follows standard Go conventions as outlined in [Effective Go](https://go.dev/doc/effective_go) and the [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments).

#### Naming

- Use **MixedCaps** or **mixedCaps** rather than underscores for multi-word names
- Acronyms should be all capitals (e.g., `URL`, `HTTP`, `API`)
- Interfaces with a single method should be named with the method name plus the `-er` suffix (e.g., `Reader`, `Writer`)
- Package names should be short, concise, lowercase, and without underscores or mixedCaps

#### Comments

- All exported functions, types, constants, and variables must have doc comments
- Doc comments should be complete sentences starting with the name of the element
- Package comments should be included above the package declaration
- Use `//` style comments; avoid `/* */` except for package comments

Example:
```go
// LoadDriver manages the lifecycle of container loads.
// It provides methods to create, start, stop, and remove loads.
type LoadDriver interface {
    // Create creates a new load with the given configuration.
    Create(ctx context.Context, config *LoadConfig) error

    // Start starts the specified load by ID.
    Start(ctx context.Context, loadID string) error
}
```

#### Code Organization

- Organize imports into groups: standard library, third-party, local packages
- Use `make verify-fmt` to format all code before committing
- Run `make vet` to catch common mistakes

#### Error Handling

- Always check errors; don't use `_` to discard errors unless you have a good reason
- Provide context when returning errors using `fmt.Errorf` with `%w` for wrapping
- Use meaningful error messages that help debugging

Example:
```go
if err := daemon.Start(ctx); err != nil {
    return fmt.Errorf("failed to start daemon: %w", err)
}
```

### Project Structure

```
realm/
├── cmd/                    # Command-line interface
│   ├── main.go            # Application entry point
│   ├── daemon.go          # Daemon commands
│   ├── containers.go      # Container management commands
│   ├── images.go          # Image management commands
│   ├── network.go         # Network commands
│   ├── nodes.go           # Node management commands
│   ├── proxy.go           # Proxy commands
│   └── loads.go           # Load management commands
├── daemon/                # Daemon implementation
├── clib/                  # C library bindings
│   ├── client/            # C client library
│   └── daemon/            # C daemon library
├── drivers/               # Standard drivers
├── internal/              # Private application code
│   ├── dto/               # Data Transfer Objects
│   └── runtime/           # Runtime abstractions
├── config/                # Configuration management
│   └── logs/              # Logging configuration
├── dev/                   # Development tools and scripts
│   └── ansible/           # Ansible playbooks for deployment
└── docs/                  # Documentation
```

### Development Workflow

1. **Fork and clone** the repository
2. **Create a feature branch** from `main`:
   ```bash
   git checkout -b feature/my-feature
   ```
3. **Make your changes** following the code style guidelines
4. **Write tests** for new functionality
5. **Run tests** to ensure everything passes:
   ```bash
   make test
   ```
6. **Format your code**:
   ```bash
   gofmt -w .
   ```
7. **Commit your changes** with clear, descriptive commit messages
8. **Push to your fork** and submit a pull request

### Testing

- Place tests in `*_test.go` files in the same package
- Run tests before submitting pull requests:
  ```bash
  make test
  ```
### Documentation

- Add doc comments to all exported types, functions, constants, and variables
- Keep comments up-to-date when changing code
- Use examples in doc comments where helpful

### Pull Request Guidelines

- Keep pull requests focused on a single feature or bug fix
- Reference any related issues in the PR description
- Ensure all tests pass and code is formatted before submitting
- Be responsive to review feedback
- Squash commits if requested before merging

