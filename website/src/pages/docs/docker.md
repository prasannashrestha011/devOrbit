---
layout: ../../layouts/Docs.astro
label: Docker integration
title: Docker integration
description: Discover Docker Compose projects and manage their services from your workspace.
source: server/internal/modules/dockercompose/manager.go
---

## Connect a Compose project

Install Docker with Compose v2 and start the Docker daemon. DevOrbit itself runs natively; Docker is needed only for Compose projects.

DevOrbit looks for `compose.yaml`, `compose.yml`, `docker-compose.yaml`, or `docker-compose.yml` in the workspace root and configured service working directories. Matching Compose override files are included. When several configurations are found, choose one in the **Compose file** selector.

For a project outside the workspace root, configure a service's `cwd` to point to that project directory. See [workspaces](/docs/workspaces/#connect-an-existing-project).

## Manage containers

The Docker panel shows defined services, container states, published ports, and health information when available.

| Action | What it does |
| --- | --- |
| Up | Creates or recreates containers and may build images |
| Start stopped | Starts existing stopped containers |
| Stop | Stops containers without removing them |
| Restart | Restarts running containers |
| Build images | Builds services that define a build configuration |
| Down | Removes the project's containers and networks, keeping volumes |

**Down** asks for confirmation. Check the selected project before confirming. Operations report their progress and errors in the panel; completion of a command does not guarantee that a container is healthy.

## Inspect a service

Open **Logs** or expand a service's details to inspect its containers. Check the reported state and health when a container is restarting or unhealthy, then inspect its logs for the cause.

Use **Refresh** after changing Compose configuration or Docker availability. If the panel has no project to show, check the file location and the configured working directories. See [Docker troubleshooting](/docs/troubleshooting/#docker-is-unavailable).
