# Realm - Lightweight orchestration service

Realm is a lightweight, embeddable, and extensible open-source orchestration service.

It's lightweight because it ships as a single executable: one binary runs as an agent on each node in the cluster and also serves as the command-line client.

It's extensible through a driver system, which lets you add support for custom workloads or node types.

It's embeddable via a C API, so other programs can drive a cluster directly.

## Quick Installation

**Linux / macOS:**

```sh
curl -fsSL https://realm.bitomia.com/install.sh | sudo sh
```

**Windows (PowerShell):**

```sh
irm https://realm.bitomia.com/install.ps1 | sudo iex
```

The install location can be overridden via the `REALM_INSTALL_DIR` environment variable.

## Getting Started

Realm running as client does not require any external dependencies. For detailed agent installation on Linux or Windows, see the [Getting Started Guide](https://bitomia.com/realm).

## Documentation

Documentation can be found at the [Realm documentation site](https://bitomia.com/realm).

## Contributing

Please follow the [contributing guidelines](docs/contributing.md) to ensure code quality and consistency.

## Project Structure

```
realm/
├── cmd/                   # Command-line interface
├── agent/                 # Agent implementation
├── drivers/               # Standard drivers
├── internal/              # Private application code
├── config/                # Configuration management
└── dev/                   # Development tools and scripts
```

## License

Realm is **dual-licensed**:

### Open Source License: AGPL-3.0

The open-source version of Realm is licensed under the **GNU Affero General Public License v3.0 (AGPL-3.0)**.
