```mermaid
stateDiagram-v2
    [*] --> Ready: Provision

    Ready --> Running: Run
    Ready --> [*]: Deprovision

    Running --> Stopped: Stop

    Stopped --> Running: Run
    Stopped --> [*]: Deprovision

    Error --> [*]: Deprovision
```
