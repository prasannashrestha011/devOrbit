import { useCallback, useEffect, useState } from "react";
import { Box, ChevronDown, FileCode2, FileText, Hammer, MoreHorizontal, Play, RefreshCw, RotateCw, Square, Trash2 } from "lucide-react";
import { api, type DockerAction, type DockerOperation, type DockerSnapshot } from "./api";
import DockerServiceDetails, { type DockerServiceTab } from "./DockerServiceDetails";

function message(error: unknown) { return error instanceof Error ? error.message : String(error); }

export default function DockerPanel({ workspaceId }: { workspaceId: string }) {
  const storageKey = `devorbit.docker.file.${workspaceId}`;
  const [file, setFile] = useState(() => localStorage.getItem(storageKey) ?? "");
  const [snapshot, setSnapshot] = useState<DockerSnapshot | null>(null);
  const [loading, setLoading] = useState(true);
  const [pending, setPending] = useState(false);
  const [operation, setOperation] = useState<DockerOperation | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [detail, setDetail] = useState<{ service: string; tab: DockerServiceTab } | null>(null);
  const [servicesExpanded, setServicesExpanded] = useState(true);

  const refresh = useCallback(async () => {
    const current = await api.dockerSnapshot(workspaceId, file);
    setSnapshot(current);
    return current;
  }, [workspaceId, file]);

  useEffect(() => {
    let active = true;
    setLoading(true);
    api.dockerSnapshot(workspaceId, file)
      .then((value) => { if (active) setSnapshot(value); })
      .catch((problem) => { if (active) setError(message(problem)); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, [workspaceId, file]);

  useEffect(() => {
    if (!operation || operation.state !== "running") return;
    let active = true;
    const timer = window.setInterval(() => {
      api.dockerOperation(workspaceId, operation.id).then((latest) => {
        if (!active) return;
        setOperation(latest);
        if (latest.state === "failed") setError(`${latest.action} failed: ${latest.error ?? "Check Docker and retry."}`);
        if (latest.state === "succeeded") setNotice(`${latest.action} completed. Container health is shown below.`);
        if (latest.state !== "running") refresh().catch((problem) => setError(message(problem)));
      }).catch((problem) => { if (active) setError(message(problem)); });
    }, 1000);
    return () => { active = false; window.clearInterval(timer); };
  }, [operation, workspaceId, refresh]);

  const hasRestartingContainers = snapshot?.project?.services.some((service) => service.containers.some((container) => container.state.toLowerCase() === "restarting")) ?? false;
  const hasLiveContainers = snapshot?.availability.state === "ready" && !snapshot.error && snapshot.project?.services.some((service) => service.containers.some((container) => ["running", "restarting", "starting"].includes(container.state.toLowerCase())));
  useEffect(() => {
    if (!hasLiveContainers) return;
    let active = true;
    let timer: number;
    const poll = async () => {
      try {
        const latest = await api.dockerSnapshot(workspaceId, file);
        if (active) setSnapshot(latest);
      } catch (problem) {
        if (active) setError(message(problem));
      } finally {
        if (active) timer = window.setTimeout(poll, hasRestartingContainers ? 2000 : 5000);
      }
    };
    timer = window.setTimeout(poll, hasRestartingContainers ? 2000 : 5000);
    return () => { active = false; window.clearTimeout(timer); };
  }, [hasLiveContainers, hasRestartingContainers, workspaceId, file]);

  function choose(next: string) {
    setFile(next);
    localStorage.setItem(storageKey, next);
    setError(""); setNotice(""); setSnapshot(null);
    setDetail(null);
  }

  async function run(action: DockerAction, service = "") {
    if (!snapshot?.project) return;
    if (action === "down" && !window.confirm(`Remove containers and networks for Compose project “${snapshot.project.name}”? Volumes will be kept.`)) return;
    setError(""); setNotice(""); setPending(true);
    try {
      setOperation(await api.dockerAction(workspaceId, snapshot.project.file, action, service));
    } catch (problem) { setError(message(problem)); }
    finally { setPending(false); }
  }

  if (!loading && snapshot?.candidates.length === 0) return null;
  const project = snapshot?.project;
  const busy = pending || operation?.state === "running";
  const statusReady = snapshot?.availability.state === "ready" && !snapshot.error;
  const actionsDisabled = busy || !statusReady;
  const projectActive = project?.services.some((service) => service.containers.some((container) => ["running", "restarting"].includes(container.state.toLowerCase()))) ?? false;
  const projectStartable = project?.services.some((service) => service.containers.some((container) => !["running", "restarting"].includes(container.state.toLowerCase()))) ?? false;
  return <section className="docker-panel" aria-label="Docker Compose">
    <div className="docker-heading">
      <div className="docker-heading-copy">
        <span ><img src={new URL("../src-tauri/icons/docker.svg", import.meta.url).href} width={64} height={64} alt="" /></span>
        <div><span className="eyebrow">DOCKER COMPOSE</span><h2>Docker services <span className="count-pill">{project?.services.length ?? 0}</span></h2><p>Manage the Compose services connected to this workspace.</p></div>
      </div>
      <button className="button subtle" disabled={busy || loading} onClick={() => { setLoading(true); refresh().catch((problem) => setError(message(problem))).finally(() => setLoading(false)); }}><RefreshCw size={16} aria-hidden="true" /> Refresh</button>
    </div>
    {loading && !snapshot && <p className="docker-muted">Checking Compose configuration…</p>}
    {snapshot && <>
      <div className="docker-project-card">
        <div className="docker-project-info">
          {project && <>
            <div className="docker-project-title"><strong>{project.name}</strong><span>{project.services.length} services</span></div>
            <div className="docker-project-stats">
              {statusReady ? <><span><strong>{project.created}</strong> created</span><span><strong>{project.running}</strong> running</span><span><strong>{project.stopped}</strong> stopped</span></> : <span>Status unavailable</span>}
            </div>
          </>}
          <div className="docker-file"><FileCode2 size={17} aria-hidden="true" />
            <div><span className="docker-file-label">Compose file</span>
              {snapshot.candidates.length > 1 ? <select className="docker-select" aria-label="Compose file" value={file} onChange={(event) => choose(event.target.value)} disabled={busy}>
                <option value="">Select a configuration</option>
                {snapshot.candidates.map((item) => <option key={item.file} value={item.file}>{item.files.join(" + ")}</option>)}
              </select> : <span title={snapshot.candidates[0]?.files.join(" + ")}>{snapshot.candidates[0]?.files.join(" + ")}</span>}
            </div>
          </div>
          <p className={`docker-availability ${snapshot.availability.state === "ready" ? "ready" : ""}`}><span className="docker-dot" />{snapshot.availability.message}</p>
          {project && !statusReady && <span className="docker-muted">Container status unavailable</span>}
        </div>
        {project && <div className="docker-project-controls"><div className="docker-actions">
          <button className="button primary" disabled={actionsDisabled} onClick={() => run("up")} title="Create or recreate containers; may build images"><Play size={15} aria-hidden="true" /> Up</button>
          <button className="button" disabled={actionsDisabled || !projectActive} onClick={() => run("stop")}><Square size={15} aria-hidden="true" /> Stop</button>
          <details className="docker-menu" onBlur={(event) => { if (!event.currentTarget.contains(event.relatedTarget)) event.currentTarget.removeAttribute("open"); }}><summary className="button" aria-label="More project actions" title="More project actions"><MoreHorizontal size={18} aria-hidden="true" /></summary><div className="docker-menu-list">
            <button disabled={actionsDisabled || !projectStartable} onClick={(event) => { event.currentTarget.closest("details")?.removeAttribute("open"); void run("start"); }}><Play size={15} aria-hidden="true" /> Start stopped</button>
            <button disabled={actionsDisabled || project.running === 0} onClick={(event) => { event.currentTarget.closest("details")?.removeAttribute("open"); void run("restart"); }}><RotateCw size={15} aria-hidden="true" /> Restart</button>
            <button disabled={actionsDisabled || !project.services.some((item) => item.build)} onClick={(event) => { event.currentTarget.closest("details")?.removeAttribute("open"); void run("build"); }}><Hammer size={15} aria-hidden="true" /> Build images</button>
            <button className="danger" disabled={actionsDisabled || project.created === 0} title="Remove containers and networks; keep volumes" onClick={(event) => { event.currentTarget.closest("details")?.removeAttribute("open"); void run("down"); }}><Trash2 size={15} aria-hidden="true" /> Down…</button>
          </div></details>
        </div></div>}
      </div>
      {snapshot.error && <p className="docker-error" role="alert">{snapshot.error}</p>}
      {project && <div className="docker-services">
        <div className="docker-services-heading"><h3><button type="button" className="docker-services-toggle" aria-expanded={servicesExpanded} onClick={() => setServicesExpanded((expanded) => !expanded)}>Compose services <ChevronDown size={16} aria-hidden="true" /></button></h3><span>{project.services.length} defined in {project.name}</span></div>
        {servicesExpanded && project.services.map((service) => {
          const running = service.containers.filter((container) => container.state.toLowerCase() === "running").length;
          const restarting = service.containers.some((container) => container.state.toLowerCase() === "restarting");
          const unhealthy = service.containers.some((container) => container.health.toLowerCase() === "unhealthy");
          const failed = service.containers.some((container) => ["dead", "failed"].includes(container.state.toLowerCase()));
          const starting = service.containers.some((container) => ["created", "starting"].includes(container.state.toLowerCase()));
          const state = !statusReady ? "Unavailable" : service.containers.length === 0 ? "Not created" : failed ? "Error" : restarting ? "Restarting" : starting ? "Starting" : running === service.containers.length ? "Running" : running > 0 ? "Partially running" : service.containers.every((container) => container.state.toLowerCase() === "exited") ? "Exited" : "Stopped";
          const stateStyle = state === "Running" ? "running" : state === "Restarting" ? "restarting" : state === "Starting" ? "restarting" : state === "Error" ? "error" : state === "Stopped" || state === "Exited" ? "stopped" : "unavailable";
          const ports = service.containers.flatMap((container) => container.ports);
          const displayedPorts = ports.length ? ports : service.ports;
          const canStart = service.containers.length > 0 && running < service.containers.length && !restarting;
          const canStop = running > 0 || restarting || starting;
          const primaryAction: DockerAction = canStop ? "stop" : canStart ? "start" : "up";
          const expanded = detail?.service === service.name;
          return <article className={`docker-service ${expanded ? "expanded" : ""}`} key={service.name}>
            <div className="docker-service-identity"><span className="docker-service-icon"><Box size={20} strokeWidth={1.7} aria-hidden="true" /></span><div><h4>{service.name}</h4><small className="docker-image" title={service.image || undefined}>{service.image || (service.build ? "Built locally" : "No image specified")}</small><div className="docker-container-names">{service.containers.length ? service.containers.map((container) => <code key={container.id} title={container.id}>{container.name}</code>) : <span>No container created</span>}</div></div></div>
            <div className="docker-service-facts"><div><span className="docker-fact-label">STATUS</span><span className={`docker-state ${stateStyle}`}><span className="docker-dot" />{state}</span>{service.containers.length > 1 && <small>{running} of {service.containers.length} running</small>}{service.containers.some((container) => container.health) && <small className={unhealthy ? "docker-health-error" : "docker-health-good"}>Health: {service.containers.map((container) => container.health).filter(Boolean).join(", ")}</small>}</div><div><span className="docker-fact-label">PORTS</span><span className="docker-port-value">{displayedPorts.length ? [...new Set(displayedPorts.map((port) => `${port.published ? `${port.host || "0.0.0.0"}:${port.published} → ` : ""}${port.target}/${port.protocol || "tcp"}`))].join(", ") : "—"}</span></div></div>
            <div className="docker-service-actions">
              <button className={`button ${primaryAction === "up" ? "primary" : ""}`} aria-label={`${primaryAction} ${service.name}`} disabled={actionsDisabled} onClick={() => run(primaryAction, service.name)}>{primaryAction === "stop" ? <Square size={14} aria-hidden="true" /> : <Play size={14} aria-hidden="true" />}{primaryAction === "up" ? "Up" : primaryAction === "stop" ? "Stop" : "Start"}</button>
              <button className="button" aria-label={`View logs for ${service.name}`} onClick={() => setDetail({ service: service.name, tab: "logs" })}><FileText size={14} aria-hidden="true" /> Logs</button>
              <details className="docker-menu" onBlur={(event) => { if (!event.currentTarget.contains(event.relatedTarget)) event.currentTarget.removeAttribute("open"); }}><summary className="button" aria-label={`More actions for ${service.name}`} title={`More actions for ${service.name}`}><MoreHorizontal size={18} aria-hidden="true" /></summary><div className="docker-menu-list">
                <button disabled={actionsDisabled} onClick={(event) => { event.currentTarget.closest("details")?.removeAttribute("open"); void run("up", service.name); }}><Play size={15} aria-hidden="true" /> Up / recreate</button>
                <button disabled={actionsDisabled || !canStart} onClick={(event) => { event.currentTarget.closest("details")?.removeAttribute("open"); void run("start", service.name); }}><Play size={15} aria-hidden="true" /> Start</button>
                <button disabled={actionsDisabled || !canStop} onClick={(event) => { event.currentTarget.closest("details")?.removeAttribute("open"); void run("stop", service.name); }}><Square size={15} aria-hidden="true" /> Stop</button>
                <button disabled={actionsDisabled || running === 0 || restarting} onClick={(event) => { event.currentTarget.closest("details")?.removeAttribute("open"); void run("restart", service.name); }}><RotateCw size={15} aria-hidden="true" /> Restart</button>
                {service.build && <button disabled={actionsDisabled} onClick={(event) => { event.currentTarget.closest("details")?.removeAttribute("open"); void run("build", service.name); }}><Hammer size={15} aria-hidden="true" /> Build image</button>}
              </div></details>
              <button className="button docker-expand-button" aria-label={`${expanded ? "Hide" : "Show"} details for ${service.name}`} aria-expanded={expanded} onClick={() => setDetail((current) => current?.service === service.name ? null : { service: service.name, tab: "overview" })}><ChevronDown size={17} aria-hidden="true" /></button>
            </div>
            {(state === "Error" || state === "Restarting" || unhealthy) && <p className={`docker-service-warning ${state === "Error" || unhealthy ? "error" : ""}`} role="status">{state === "Restarting" ? "Container is restarting. Open details to check health and logs." : unhealthy ? "Health check is failing. Open details to inspect the last failure and logs." : "Container failed. Open details for the Docker error and logs."}</p>}
            {expanded && <DockerServiceDetails workspaceId={workspaceId} file={project.file} service={service} tab={detail.tab} onTabChange={(tab) => setDetail({ service: service.name, tab })} />}
          </article>;
        })}
      </div>}
    </>}
    {busy && <p role="status" className="docker-muted">{pending ? "Starting Docker operation…" : `${operation?.action} in progress…`} {operation?.state === "running" && <button className="text-button" onClick={() => api.cancelDockerOperation(workspaceId, operation.id).catch((problem) => setError(message(problem)))}>Cancel</button>}</p>}
    {notice && <p role="status" className="docker-availability ready">{notice}</p>}
    {error && <p role="alert" className="docker-error">{error}</p>}
  </section>;
}
