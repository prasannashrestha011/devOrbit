# DevOrbit architecture

```mermaid
flowchart TB
    Client["Local client"] --> API["DevOrbit API"]

    subgraph Backend["Backend modules"]
        Workspace["Workspace"]
        Service["Service configuration"]
        Orchestration["Workspace orchestration"]
        Process["Process supervisor"]
        Terminal["PTY terminal sessions"]
        Logs["Logs"]
        Events["Events / SSE"]
    end

    API --> Workspace
    API --> Service
    API --> Process
    API --> Terminal --> Process
    API --> Logs
    API --> Events

    Workspace --> Orchestration --> Process
    Process -- loads configuration --> Service
    Process --> Apps["Managed apps and workers"]
    Apps -- PTY output --> Process --> Logs
    Process -- lifecycle events --> Events
    Logs -- log events --> Events

    Workspace -- metadata --> DB[(SQLite)]
    Service -- stable key to UUID mapping --> DB
    Service -- read and write --> YAML[".devorbit/workspace.yaml"]
    Logs --> Files["Local log files"]
```

Service configuration lives only in each workspace's `.devorbit/workspace.yaml`. SQLite holds workspace metadata and service identities for stable UUID and log associations. The API validates YAML before saving it atomically, and each start loads a fresh validated snapshot. Process state and live events stay in memory; logs are stored in local files. A worker is a managed service, not a separate DevOrbit module.

Each service execution owns a PTY session. The Go API exposes its raw output, input, and resize through an authenticated WebSocket scoped to the service and execution ID. Tauri keeps the bearer token in Rust and forwards output to xterm.js through a channel. The supervisor drains output even without a viewer and retains 1 MiB for reconnect replay. New log entries use the merged, readable `terminal` stream; older `stdout` and `stderr` entries remain available.
