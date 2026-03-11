```mermaid
stateDiagram-v2
    [*] --> Ready: Provision

    Ready --> Running: Start
    Ready --> [*]: Deprovision

    Running --> Stopped: Stop

    Stopped --> Running: Start
    Stopped --> [*]: Deprovision

    Error --> [*]: Deprovision
```
