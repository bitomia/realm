```mermaid
stateDiagram-v2
    [*] --> Planned: PlanDeployment
    
    Planned --> Running: RunDeployment
    Planned --> [*]: UnplanDeployment

    Stopped --> Running: RunDeployment
    Stopped --> [*]: UnplanDeployment

    Running --> Stopped: StopDeployment

    Error --> [*]: UnplanDeployment
```
