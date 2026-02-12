```mermaid
stateDiagram-v2
    [*] --> Planned: Plan
    Planned --> [*]: Unplan
    Error --> [*]: Unplan
```
