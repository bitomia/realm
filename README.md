# Realm - Simple orchestration service

Realm is a extendable, embeddable and simple orchestration service for different type of loads such as native processes or OCI containers.

It's simple because it is just one executable to command the cluster where Realm runs as daemon on each one of the cluster nodes. It's also extendable because it uses a driver systems to extend it with custom loads or node drivers. Managing the cluster can be commanded from command-line interface or using the REST API that each daemon exposes. It's embeddable because Realm provides a C API to interface with clusters.

## Quick Installation

**Linux:**

```sh
curl -fsSL https://raw.githubusercontent.com/bitomia/realm/main/install.sh | sh
```

**Windows (PowerShell):**

```powershell
irm https://raw.githubusercontent.com/bitomia/realm/main/install.ps1 | iex
```

The install location can be overridden via the `REALM_INSTALL_DIR` environment variable.

## Getting Started

Realm running as client does not require any external dependencies. For detailed daemon installation on Linux or Windows, see the [Getting Started Guide](docs/getting-started.md).

## Documentation

- [Getting Started](docs/getting-started.md) - Installation and setup for Linux and Windows
- [Configuration](docs/configuration.md) - Configuration reference (daemon, nodes, loads, etcd, registries, discovery, environment variables)
- [Container Driver](docs/container-driver.md) - Container driver: entrypoint, volumes, networking
- [Process Driver](docs/process-driver.md) - Process driver: commands, signals, lifecycle
- [Development Guide](docs/development-guide.md) - Development environment setup
- [Contributing](docs/contributing.md) - Contributing guidelines

## Configuration

Realm is configured through a YAML file with four top-level sections: `daemon`, `nodes`, `loads`, and `discovery`. Configuration values can also be set via environment variables with the `REALM_` prefix. See the full [Configuration Reference](docs/configuration.md) for details.

## Contributing

Please follow the [contributing guidelines](docs/contributing.md) to ensure code quality and consistency.

## Project Structure

```
realm/
├── cmd/                   # Command-line interface
├── daemon/                # Daemon implementation
├── drivers/               # Standard drivers
├── internal/              # Private application code
├── config/                # Configuration management
├── dev/                   # Development tools and scripts
└── docs/                  # Documentation
```

## Development Environment

See the [Development Guide](docs/development-guide.md) for setting up your development environment on Debian 12 or Windows 11 Pro.

## License

Realm is **dual-licensed**:

### Open Source License: AGPL-3.0

The open-source version of Realm is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**.
