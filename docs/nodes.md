```mermaid
stateDiagram-v2
    [*] --> Online: Startup
    Online --> Ready: Provision
    Ready --> Online: Deprovision
    Online --> [*]: Shutdown
    Error --> [*]: Deprovision
```
