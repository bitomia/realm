```mermaid
stateDiagram-v2
    [*] --> Online: PowerOn
    Online --> Ready: Register
    Ready --> Online: Unregister
    Online --> [*]: Shutdown/PowerOff
    Error --> [*]: Unregister
```
