# Process Driver

The process driver manages native OS processes as loads. It allows you to run executables directly on the host system with full control over arguments, working directory, and process lifecycle.

## Features

- Configure command-line arguments
- Set working directory for process execution
- Graceful shutdown with configurable signals
- Force kill option for unresponsive processes
- Automatic stdout/stderr logging

## Configuration

### Example

```yaml
loads:
  node_app:
    node: lab1
    driver: process
    driver_config:
      start_cmd: node
      start_args: "server.js"
      working_dir: /opt/myapp
      stop_signal: SIGTERM
```

### Process with Force Kill

For processes that may not respond to signals gracefully:

```yaml
loads:
  stubborn_service:
    node: lab1
    driver: process
    driver_config:
      start_cmd: legacy_app
      stop_signal: SIGTERM
      force_kill: true
```

When `force_kill` is enabled, if the process doesn't exit within 3 seconds after receiving the stop signal, it will be forcefully killed with SIGKILL.

## Configuration Reference

### ProcessConfig Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `start_cmd` | string | Yes | Executable name or path. Must be in PATH or absolute path. Cannot contain arguments (use `start_args` instead). |
| `start_args` | string | No | Command-line arguments passed to the executable. Arguments are split by whitespace. |
| `working_dir` | string | No | Working directory for the process. Must exist before starting. |
| `stop_signal` | string | No | Signal sent to stop the process gracefully. Default: SIGHUP |
| `force_kill` | bool | No | If true, sends SIGKILL after 3 second timeout. Default: false |

### Supported Signals

The following signals can be used for `stop_signal`:

| Signal | Description |
|--------|-------------|
| `SIGTERM` | Termination signal (graceful shutdown) |
| `SIGINT` | Interrupt signal (Ctrl+C) |
| `SIGKILL` | Kill signal (cannot be caught) |
| `SIGHUP` | Hangup signal |
| `SIGQUIT` | Quit signal |
| `SIGUSR1` | User-defined signal 1 |
| `SIGUSR2` | User-defined signal 2 |
| `SIGABRT` | Abort signal |
| `SIGALRM` | Alarm signal |
| `SIGILL` | Illegal instruction |
| `SIGPWR` | Power failure |
| `SIGSTOP` | Stop signal |
| `SIGTRAP` | Trap signal |
