# Realm C Library

This directory contains the C-exportable library for the Realm daemon.

## Overview

The Realm daemon handlers have been refactored to support C exports, allowing the daemon functionality to be called from other languages via FFI (Foreign Function Interface).

## Architecture

The refactoring follows a layered architecture:

```
┌─────────────────────────────────────┐
│   C Applications / FFI Bindings     │
│   (Python, Ruby, Node.js, etc.)     │
└─────────────────────────────────────┘
                 ↓
┌─────────────────────────────────────┐
│   lib/main.go (C Exports)           │
│   - C-compatible wrapper functions  │
│   - JSON string-based API           │
└─────────────────────────────────────┘
                 ↓
┌─────────────────────────────────────┐
│   daemon/api (Business Logic)       │
│   - Pure Go functions               │
│   - No HTTP dependencies            │
└─────────────────────────────────────┘
                 ↓
┌─────────────────────────────────────┐
│   daemon/handlers (HTTP Handlers)   │
│   - HTTP request/response handling  │
│   - Calls daemon/api functions      │
└─────────────────────────────────────┘
```

### Key Components

1. **daemon/api/api.go**: Contains the business logic separated from HTTP handling
   - Pure Go functions that can be called from both HTTP handlers and C exports
   - Standardized error handling
   - No dependency on http package

2. **lib/main.go**: C-exportable wrapper functions
   - Converts between C types (*C.char) and Go types
   - Uses JSON for complex data structures
   - Exports functions with `//export` directive

3. **daemon/handlers**: Refactored HTTP handlers
   - Now call daemon/api functions instead of implementing logic directly
   - Eliminates code duplication

## Building

To build the C shared library:

```bash
go build -buildmode=c-shared -o lib/librealm.so ./lib
```

This generates two files:
- `lib/librealm.so`: The shared library
- `lib/librealm.h`: The C header file with function declarations

## Exported Functions

All exported functions return JSON strings with the following format:

```json
{
  "success": true,
  "data": { ... },
  "error": ""
}
```

On error:
```json
{
  "success": false,
  "data": null,
  "error": "error message"
}
```

### Available Functions

#### `void StartDaemon(void)`
Starts the Realm daemon (blocking call).

#### `char* GetVersion(void)`
Returns the daemon version.

**Example response:**
```json
{
  "success": true,
  "data": {"version": "1.0.0"},
  "error": ""
}
```

#### `char* GetHealthStatus(void)`
Returns health status of all monitored services.

#### `char* GetNodeStatus(void)`
Returns current node status (CPU, memory, etc.).

#### `char* ListContainers(void)`
Returns a list of all containers with their status.

#### `char* CreateContainer(char* containerName, char* optsJSON)`
Creates a new container.

**Parameters:**
- `containerName`: Name of the container
- `optsJSON`: JSON string with container options

**Example optsJSON:**
```json
{
  "image": "docker.io/library/ubuntu:latest",
  "env": ["KEY=value"]
}
```

#### `char* UpdateContainerState(char* containerName, char* optsJSON)`
Updates the state of a container (start, stop, etc.).

**Parameters:**
- `containerName`: Name of the container
- `optsJSON`: JSON string with update options

#### `char* RemoveContainer(char* containerName, char* optsJSON)`
Removes a container.

**Parameters:**
- `containerName`: Name of the container
- `optsJSON`: JSON string with deletion options

## Memory Management

**IMPORTANT**: All string pointers returned by the C functions are allocated by Go and must be freed by the caller using the standard C `free()` function.

Example in C:
```c
char* result = GetVersion();
// Use result...
free(result);
```

## Usage Example (C)

```c
#include <stdio.h>
#include <stdlib.h>
#include "librealm.h"

int main() {
    // Get version
    char* version = GetVersion();
    printf("Version: %s\n", version);
    free(version);

    // List containers
    char* containers = ListContainers();
    printf("Containers: %s\n", containers);
    free(containers);

    // Create a container
    const char* opts = "{\"image\":\"ubuntu:latest\"}";
    char* result = CreateContainer("my-container", opts);
    printf("Result: %s\n", result);
    free(result);

    return 0;
}
```

Compile:
```bash
gcc -o example example.c -L. -lrealm
LD_LIBRARY_PATH=. ./example
```

## Adding New Exported Functions

To add a new exported function:

1. Add the business logic function in `daemon/api/api.go`
2. Add the C wrapper in `lib/main.go` with `//export` directive
3. Optionally create an HTTP handler in `daemon/handlers` that calls the API function
4. Rebuild the library

Example:

```go
// In daemon/api/api.go
func DoSomething(param string) (string, error) {
    // Business logic here
    return "result", nil
}

// In lib/main.go
//export DoSomething
func DoSomething(param *C.char) *C.char {
    goParam := C.GoString(param)
    result, err := api.DoSomething(goParam)
    if err != nil {
        return C.CString(api.ResponseToJSON(false, nil, err.Error()))
    }
    return C.CString(api.ResponseToJSON(true, result, ""))
}
```

## Benefits of This Refactoring

1. **Code Reuse**: Business logic is shared between HTTP and C APIs
2. **Maintainability**: Logic is centralized in daemon/api
3. **Testability**: API functions can be tested independently
4. **Flexibility**: Easy to add new export functions
5. **Language Interoperability**: Can be called from any language with C FFI support
