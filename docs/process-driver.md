# Process Driver

The process driver manages native OS processes as loads. It allows you to run executables directly on the host system with full control over arguments, working directory, and process lifecycle.

## Features

- Configure command-line arguments
- Set working directory for process execution
- Graceful shutdown with configurable signals
- Process identification by name (useful for wrapper-launched apps)
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

### Identifying the Process by Name

Some launchers (UWP apps, shell wrappers) spawn the real process under a different PID than the one Realm starts. Set `use_process_name: true` so that stop and status checks locate the process by its executable name instead of by stored PID:

```yaml
loads:
  uwp_app:
    node: lab1
    driver: process
    driver_config:
      start_cmd: MyUWPApp.exe
      use_process_name: true
```

The lookup matches the basename of `start_cmd` against running process names (case-insensitive on Windows).

## Configuration Reference

### ProcessConfig Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `start_cmd` | string | Yes | Executable name or path. Resolved in priority order: (1) absolute path, (2) relative to `working_dir`, (3) PATH lookup. Cannot contain arguments (use `start_args` instead). |
| `start_args` | string | No | Command-line arguments passed to the executable. Arguments are split by whitespace. |
| `working_dir` | string | No | Working directory for the process. Must exist before starting. |
| `stop_signal` | string | No | Signal sent to stop the process. Default: `SIGTERM` (Unix) or `Kill` (Windows). |
| `use_process_name` | bool | No | If true, identifies the process by executable name instead of PID (for stop and status checks). Useful for UWP apps and wrapper launchers. Default: false. |

### Supported Signals

The following signals can be used for `stop_signal`:

#### Unix systems

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


#### Windows systems

| Signal | Description |
|--------|-------------|
| `WM_CLOSE` | Posts WM_CLOSE to all top-level windows owned by the process (windowed apps only). If no window is found, falls back to terminating the process. |

If `stop_signal` is omitted on Windows, the process is terminated directly (equivalent to `proc.Kill()`).
