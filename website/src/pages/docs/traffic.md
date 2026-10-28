---
layout: ../../layouts/Docs.astro
label: Traffic inspector
title: Traffic inspector
description: Inspect local HTTP requests and responses through a temporary gateway.
source: server/README.md
---

## Start an inspection session

Start your service and open its traffic inspector. DevOrbit detects listening TCP ports from the process and its children. Choose the HTTP port when more than one is available; DevOrbit checks that it responds to HTTP before opening an inspection gateway.

Send requests to the gateway URL shown in the inspector. Requests sent directly to the original service port bypass inspection.

## Read an exchange

The inspector records the request path, query, headers, cookies, and body, along with the forwarded response's status, headers, trailers, and body. Use the exchange details to investigate what your client sent and what your service returned.

The detail API returns body previews up to 256 KiB. Body download endpoints provide all retained bytes. HTTPS interception and WebSocket frame capture are not supported.

## Manage captured data

Captures are stored privately under `DEVORBIT_DATA_DIR/traffic`. History survives disabling inspection and restarting DevOrbit. Clear it in the inspector when you no longer need it.

Each session retains its newest 500 exchanges for up to seven days. All sessions share a 512 MiB storage quota. When that quota is reached, forwarding continues, but a capture may contain only part of a body.

<aside class="callout">Requests and responses can contain credentials or other secrets. Keep the data directory private and clear sensitive captures when you finish.</aside>

If no traffic appears, check that the service is running and your client is using the gateway URL. See [troubleshooting](/docs/troubleshooting/#the-inspector-is-empty).
