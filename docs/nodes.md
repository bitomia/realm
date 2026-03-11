```mermaid
stateDiagram-v2
    [*] --> Online: Start
    Online --> Ready: Provision
    Ready --> Online: Deprovision
    Online --> [*]: Shutdown
    Error --> [*]: Deprovision
```
