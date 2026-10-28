---
layout: ../../layouts/Docs.astro
label: Workspaces
title: Workspaces
description: Group the services for a project and keep their configuration in one place.
source: server/internal/modules/workspace/service/service.go
---

## Create a workspace

Create a workspace from the desktop and give it a name and description. DevOrbit creates a managed root under `DEVORBIT_DATA_DIR/workspaces` and initializes `.devorbit/workspace.yaml` there.

The desktop shows the workspace configuration path. Use that root when resolving relative working directories; creating a workspace does not clone or move an existing project.

## Connect an existing project

Add a service through the service form or **Workspace YAML** editor. Set `cwd` to the absolute path of your existing project when it lives outside the managed workspace root.

```yaml
services:
  web:
    command: npm run dev
    cwd: /home/you/projects/web
```

Replace the example directory with your own project path. On Windows, a YAML single-quoted path such as `'C:\Users\you\projects\web'` keeps backslashes literal. Install the project's dependencies before starting the service.

For projects inside the workspace root, use a relative directory. Read [commands and directories](/docs/configuration/#commands-and-directories) for validation rules.

## Organize your workflow

Select a workspace in the sidebar to see its services. Use workspace controls to start, stop, or restart them together, or control one service at a time. A running process is not necessarily ready to receive requests.

Open **Terminals** for independent shell sessions, or open a service terminal to interact with its current execution. **Quick controls** provides an always-on-top window that follows the selected workspace.

Continue with [workspace configuration](/docs/configuration/) to edit YAML, or [services and terminals](/docs/services/) for process behavior.
