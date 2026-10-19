# DevOrbit architecture

```mermaid
flowchart TB
    Client["Local client"] --> API["DevOrbit API"]

    subgraph Backend["Backend modules"]
        Workspace["Workspace"]
        Service["Service configuration"]
        Orchestration["Workspace orchestration"]
        Process["Process supervisor"]
        Logs["Logs"]
        Events["Events / SSE"]
    end

    API --> Workspace
    API --> Service
    API --> Process
    API --> Logs
    API --> Events

    Workspace --> Orchestration --> Process
    Process -- loads configuration --> Service
    Process --> Apps["Managed apps and workers"]
    Apps -- stdout / stderr --> Logs
    Process -- lifecycle events --> Events
    Logs -- log events --> Events

    Workspace --> DB[(PostgreSQL)]
    Service --> DB
    Logs --> Files["Local log files"]
```

Workspace and service configurations live in PostgreSQL. Process state and live events stay in memory; logs are stored in local files. A worker is a managed service, not a separate DevOrbit module.
