# Phase 09 security review

The review covered startup and shutdown, HTTP routing and authentication, CRUD validation and persistence, process supervision, orchestration, filesystem access, logs, and SSE. Findings below are based on code paths and focused tests, with severity tied to impact.

| Severity | Finding | Resolution |
| --- | --- | --- |
| High | The local listener accepted arbitrary HTTP Host values, leaving a DNS rebinding route to local endpoints if a token was exposed to a browser. | The production server now requires its configured loopback IP and port in Host, including on `/health`. Config supplied directly to `app.Run` is also checked. |
| High | A failed process launch stored a raw OS error in `last_error`, which could disclose an executable path or an environment value. | Launch errors are converted to a generic status and API error. |
| High | An SSE write could block indefinitely after the handler cleared the server write deadline. | Each frame gets a fresh 30-second write deadline; the 15-second heartbeat keeps idle streams active. |
| High | Token and log paths could follow links or be read with permissions inherited from unsafe locations. | Token files are opened without following the final Unix link and checked by handle. Token directories and log roots must be private; Windows uses explicit file ACLs and checks existing ACLs. Log queries skip linked files. |
| Medium | A process could exit after Stop reserved the operation but before it read the tree pointer, causing a nil dereference. | Stop accepts an already completed process. |
| Medium | A valid JSON prefix followed by more than 1 MiB of data could be accepted because decoders used `io.LimitReader`. | The API now uses `http.MaxBytesReader`, including trailing input. |
| Medium | Duplicate Origin and query parameters had ambiguous interpretation. | Multiple Origin headers and duplicate, empty, or unknown SSE/log filters are rejected. |
| Low | A process shutdown error skipped log-manager close. | Both shutdown operations now run and their errors are joined. |

The token, logs, and SSE are accessible to anyone holding the local API token. Child processes can print secrets to logs; output is not automatically redacted. Authenticated configuration writers can execute arbitrary commands under the DevOrbit user's OS account. DevOrbit does not isolate those commands.

Native Windows verification remains necessary for token directory ACLs, token-file creation and reload, Job Object ownership, descendant cleanup, and console control. Cross compilation checks API use but does not prove Windows runtime behavior. Native macOS verification remains necessary for process-group and signal behavior. Unix process groups and Windows jobs do not guarantee cleanup of intentionally detached or escaped descendants. On Unix, signaling a process group after its leader has exited has a narrow group-ID reuse risk; a future stronger ownership mechanism would be needed to eliminate it. SSE is best effort, with no replay.
