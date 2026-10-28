import { useEffect, useRef, useState } from "react";
import { Channel, invoke } from "@tauri-apps/api/core";
import { api, type DockerSnapshot, type DockerTerminal } from "./api";
import DockerError from "./DockerError";
import TerminalPane from "./TerminalPane";
import TrafficInspector from "./TrafficInspector";

type Service = NonNullable<DockerSnapshot["project"]>["services"][number];
type LogFrame = { stream: "stdout" | "stderr" | "error" | "status"; data: string };
export type DockerServiceTab = "overview" | "logs" | "terminal" | "inspect" | "traffic";

function portText(port: Service["ports"][number]) {
  const target = `${port.target}/${port.protocol || "tcp"}`;
  return port.published ? `${port.host || "0.0.0.0"}:${port.published} → ${target}` : `${target} · internal`;
}

function DockerLogs({ workspaceId, file, service }: { workspaceId: string; file: string; service: string }) {
  const [tail, setTail] = useState(200);
  const [lines, setLines] = useState<LogFrame[]>([]);
  const [paused, setPaused] = useState(false);
  const [state, setState] = useState("Connecting…");
  const [retry, setRetry] = useState(0);
  const pausedRef = useRef(paused);
  const outputRef = useRef<HTMLDivElement>(null);
  pausedRef.current = paused;

  useEffect(() => { if (!paused && outputRef.current) outputRef.current.scrollTop = outputRef.current.scrollHeight; }, [lines, paused]);

  useEffect(() => {
    const viewerId = crypto.randomUUID();
    let active = true;
    const channel = new Channel<LogFrame>();
    setLines([]);
    setState("Connecting…");
    channel.onmessage = (frame) => {
      if (!active) return;
      if (frame.stream === "status") { setState("Live"); return; }
      if (frame.stream === "error") { setState(frame.data); return; }
      if (!pausedRef.current) setLines((current) => [...current.slice(-999), frame]);
    };
    const started = invoke("start_docker_logs", { workspaceId, file, service, tail, viewerId, channel })
      .catch((problem) => { if (active) setState(String(problem)); });
    return () => {
      active = false;
      void started.finally(() => invoke("stop_docker_logs", { viewerId }).catch(() => {}));
    };
  }, [workspaceId, file, service, tail, retry]);

  return <section className="docker-detail-logs" aria-label={`${service} Docker logs`}>
    <div className="docker-detail-toolbar">
      <span className="docker-muted">{state}</span>
      <div className="docker-actions">
        <label>Tail <select aria-label="Docker log tail" value={tail} onChange={(event) => setTail(Number(event.target.value))}><option value={100}>100</option><option value={200}>200</option><option value={500}>500</option><option value={1000}>1000</option></select></label>
        <button className="button" onClick={() => setPaused((value) => !value)}>{paused ? "Resume" : "Pause"}</button>
        <button className="button" onClick={() => setLines([])}>Clear</button>
        <button className="button" onClick={() => setRetry((value) => value + 1)}>Reconnect</button>
      </div>
    </div>
    <div className="docker-log-output" role="log" aria-live="off" ref={outputRef}>{lines.length ? lines.map((line, index) => <div key={index} className={`docker-log-line ${line.stream}`}>{line.data}</div>) : <span className="docker-muted">No output yet.</span>}</div>
  </section>;
}

export default function DockerServiceDetails({ workspaceId, file, service, tab, onTabChange }: { workspaceId: string; file: string; service: Service; tab: DockerServiceTab; onTabChange: (tab: DockerServiceTab) => void }) {
  const [terminal, setTerminal] = useState<DockerTerminal | null>(null);
  const [selectedContainer, setSelectedContainer] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const terminalRef = useRef(terminal);
  terminalRef.current = terminal;

  useEffect(() => () => {
    if (terminalRef.current) void api.deleteDockerTerminal(workspaceId, terminalRef.current.id).catch(() => {});
  }, [workspaceId]);

  useEffect(() => {
    if (tab !== "terminal" && terminal) {
      setTerminal(null);
      void api.deleteDockerTerminal(workspaceId, terminal.id).catch((problem) => setError(String(problem)));
    }
  }, [tab, terminal, workspaceId]);

  async function closeTerminal() {
    if (!terminal) return;
    setBusy(true);
    try { await api.deleteDockerTerminal(workspaceId, terminal.id); setTerminal(null); }
    catch (problem) { setError(String(problem)); }
    finally { setBusy(false); }
  }

  const running = service.containers.filter((container) => container.state.toLowerCase() === "running");
  const selected = running.some((container) => container.id === selectedContainer) ? selectedContainer : running[0]?.id ?? "";
  return <div className="docker-service-detail">
    <div className="docker-detail-tabs" role="tablist" aria-label={`${service.name} details`}>
      {(["overview", "logs", "terminal", "inspect", "traffic"] as const).map((name) => <button key={name} role="tab" aria-selected={tab === name} className={tab === name ? "active" : ""} onClick={() => onTabChange(name)}>{name === "traffic" ? "HTTP traffic" : name[0].toUpperCase() + name.slice(1)}</button>)}
    </div>
    {tab === "overview" && <div className="docker-detail-overview" role="tabpanel">
      {service.containers.length ? service.containers.map((container) => <div className="docker-container-detail" key={container.id}>
        <div><strong>{container.name}</strong><code title={container.id}>{container.id.slice(0, 12)}</code><span className={`docker-state ${container.state.toLowerCase() === "running" ? "running" : container.state.toLowerCase() === "restarting" ? "restarting" : "stopped"}`}><span className="docker-dot" />{container.state}</span></div>
        <div><span>Health</span><strong className={container.health === "unhealthy" ? "docker-health-error" : container.health === "healthy" ? "docker-health-good" : ""}>{container.health || "No health check"}</strong></div>
        <div><span>Restarts</span><strong>{container.restart_count ?? 0}</strong></div>
        <div><span>Ports</span><div>{container.ports.length ? container.ports.map((port, index) => <code key={index}>{portText(port)}</code>) : <span>None exposed</span>}</div></div>
        {(container.error || container.health_error) && <p className="docker-detail-error" role="status">{container.error || container.health_error} <span>Check the Logs tab or restart this service.</span></p>}
      </div>) : <p className="docker-muted">No containers have been created for this service.</p>}
      {!service.containers.length && service.ports.length > 0 && <div className="docker-container-detail"><strong>Configured ports</strong>{service.ports.map((port, index) => <code key={index}>{portText(port)}</code>)}</div>}
    </div>}
    {tab === "logs" && <DockerLogs workspaceId={workspaceId} file={file} service={service.name} />}
    {tab === "terminal" && <div className="docker-detail-terminal" role="tabpanel">
      {terminal ? <TerminalPane key={terminal.id} docker={terminal} workspaceId={workspaceId} focused expanded busy={busy} onFocus={() => {}} onClose={() => void closeTerminal()} onStopped={() => void closeTerminal()} /> : <div className="docker-detail-terminal-start">
        {running.length ? <><label>Container <select aria-label="Terminal container" value={selected} onChange={(event) => setSelectedContainer(event.target.value)}>{running.map((container) => <option key={container.id} value={container.id}>{container.name}</option>)}</select></label><button className="button primary" disabled={busy} onClick={() => { setBusy(true); setError(""); api.createDockerTerminal(workspaceId, file, selected).then(setTerminal).catch((problem) => setError(String(problem))).finally(() => setBusy(false)); }}>Open terminal</button></> : <p className="docker-muted">Start a container to open its terminal.</p>}
      </div>}
      {error && <DockerError error={error} />}
    </div>}
    {tab === "inspect" && <pre className="docker-detail-inspect" role="tabpanel">{JSON.stringify(service, null, 2)}</pre>}
    {tab === "traffic" && <TrafficInspector workspaceId={workspaceId} kind="docker" serviceKey={service.name} file={file} ports={[...new Map(service.containers.filter((container) => container.state === "running").flatMap((container) => container.ports).filter((port) => port.published && (!port.protocol || port.protocol === "tcp") && (!port.host || port.host === "0.0.0.0" || port.host.startsWith("127."))).map((port) => [port.published, { key: port.published, label: `${port.published} → ${port.target}/tcp` }])).values()]} />}
  </div>;
}
