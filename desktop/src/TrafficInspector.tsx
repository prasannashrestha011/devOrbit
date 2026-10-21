import { useCallback, useEffect, useRef, useState } from "react";
import { listen } from "@tauri-apps/api/event";
import { ApiError, api, type TrafficRecord, type TrafficSession } from "./api";

type Port = { key: string; label: string };
type Event = { type: string; workspace_id: string; data?: { session_id?: string } };

export default function TrafficInspector({ workspaceId, kind, serviceKey, file, ports }: { workspaceId: string; kind: "local" | "docker"; serviceKey: string; file?: string; ports: Port[] }) {
  const [sessions, setSessions] = useState<TrafficSession[]>([]);
  const [sessionId, setSessionId] = useState("");
  const [port, setPort] = useState(ports[0]?.key ?? "");
  const [records, setRecords] = useState<TrafficRecord[]>([]);
  const [selected, setSelected] = useState("");
  const [method, setMethod] = useState("");
  const [status, setStatus] = useState("");
  const [search, setSearch] = useState("");
  const [paused, setPaused] = useState(false);
  const [stream, setStream] = useState("Connecting…");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const cursor = useRef(0);
  const pausedRef = useRef(false);
  const syncRef = useRef<() => void>(() => {});
  const session = sessions.find((item) => item.id === sessionId);

  useEffect(() => {
    if (!ports.some((item) => item.key === port)) setPort(ports[0]?.key ?? "");
  }, [ports, port]);

  useEffect(() => {
    let active = true;
    setSessions([]); setSessionId(""); setPort(ports[0]?.key ?? ""); setError("");
    api.trafficSessions(workspaceId).then((items) => {
      if (!active) return;
      const matched = items.filter((item) => item.target.kind === kind && item.target.service_key === serviceKey && (kind === "local" || item.target.file === file));
      setSessions(matched);
      setSessionId(matched[0]?.id ?? "");
    }).catch((problem) => { if (active) setError(String(problem)); });
    return () => { active = false; };
  }, [workspaceId, kind, serviceKey, file]);

  useEffect(() => {
    cursor.current = 0;
    setRecords([]);
    setSelected("");
    if (!sessionId) return;
    let active = true;
    let loading = false;
    let again = false;
    const sync = async () => {
      if (!active || pausedRef.current) return;
      if (loading) { again = true; return; }
      loading = true;
      try {
        do {
          again = false;
          const page = await api.trafficRecords(workspaceId, sessionId, cursor.current);
          if (!active) return;
          if (page.gap) setRecords(page.records);
          else if (page.records.length) setRecords((current) => [...current, ...page.records].slice(-500));
          if (page.records.length) cursor.current = page.records[page.records.length - 1].sequence;
          else if (cursor.current === 0) cursor.current = page.latest_sequence;
          if (page.records.length === 500 && cursor.current < page.latest_sequence) again = true;
          setError("");
        } while (again && active && !pausedRef.current);
      } catch (problem) {
        if (!active) return;
        if (problem instanceof ApiError && problem.status === 404) {
          const items = await api.trafficSessions(workspaceId).catch(() => []);
          if (active) {
            const matched = items.filter((item) => item.target.kind === kind && item.target.service_key === serviceKey && (kind === "local" || item.target.file === file));
            setSessions(matched);
            setSessionId(matched[0]?.id ?? "");
          }
        } else setError(String(problem));
      }
      finally { loading = false; if (again && active) window.setTimeout(sync, 100); }
    };
    syncRef.current = () => { void sync(); };
    const poll = window.setInterval(sync, 3000);
    const subscriptions = Promise.all([
      listen<Event>("backend-event", (event) => {
        if (event.payload.workspace_id === workspaceId && event.payload.type === "traffic.completed" && event.payload.data?.session_id === sessionId) void sync();
        if (event.payload.workspace_id === workspaceId && event.payload.type === "traffic.session" && event.payload.data?.session_id === sessionId) void api.trafficSessions(workspaceId).then((items) => {
          if (!active) return;
          const matched = items.filter((item) => item.target.kind === kind && item.target.service_key === serviceKey && (kind === "local" || item.target.file === file));
          setSessions(matched);
          setSessionId((current) => matched.some((item) => item.id === current) ? current : matched[0]?.id ?? "");
        });
      }),
      listen<string>("stream-state", (event) => { setStream(event.payload); if (event.payload === "connected") void sync(); }),
    ]);
    void sync();
    return () => { active = false; window.clearInterval(poll); void subscriptions.then((items) => items.forEach((unlisten) => unlisten())); };
  }, [workspaceId, sessionId, kind, serviceKey, file]);

  const start = useCallback(async () => {
    if (!port) return;
    setBusy(true); setError("");
    try {
      const item = await api.startTraffic(workspaceId, { kind, service_key: serviceKey, port_key: port, file });
      setSessions((current) => [...current, item]); setSessionId(item.id);
    } catch (problem) { setError(String(problem)); }
    finally { setBusy(false); }
  }, [workspaceId, kind, serviceKey, port, file]);

  const stop = async () => {
    if (!session || !window.confirm(`Disable inspection? Clients still using ${session.proxy_url} will lose access.`)) return;
    setBusy(true); setError("");
    try {
      await api.stopTraffic(workspaceId, session.id);
      const remaining = sessions.filter((item) => item.id !== session.id);
      setSessions(remaining); setSessionId(remaining[0]?.id ?? "");
    } catch (problem) { setError(String(problem)); }
    finally { setBusy(false); }
  };

  const clear = async () => {
    if (!session) return;
    try {
      await api.clearTraffic(workspaceId, session.id);
      setRecords([]); setSelected(""); cursor.current = 0; syncRef.current();
    } catch (problem) { setError(String(problem)); }
  };

  const visible = records.filter((item) => (!method || item.method === method) && (!status || String(item.status_code).startsWith(status)) && (!search || item.path.toLowerCase().includes(search.toLowerCase())));
  const detail = records.find((item) => item.id === selected);
  return <section className="traffic-inspector" aria-label="HTTP traffic inspector">
    <div className="traffic-heading"><div><span className="eyebrow">HTTP TRAFFIC</span><h2>Traffic inspector</h2><p>Only requests sent to the temporary gateway URL appear here.</p></div><span className="state state-running">{session?.state ?? "Off"}</span></div>
    {session ? <>
      {sessions.length > 1 && <label>Session <select value={sessionId} onChange={(event) => setSessionId(event.target.value)}>{sessions.map((item) => <option key={item.id} value={item.id}>{item.target.port_key}</option>)}</select></label>}
      <div className="traffic-target"><div><small>ACTUAL BACKEND</small><code>{session.upstream}</code></div><div><small>TEMPORARY GATEWAY</small><code>{session.proxy_url}</code></div><button className="button" onClick={() => navigator.clipboard.writeText(session.proxy_url).catch((problem) => setError(String(problem)))}>Copy URL</button><button className="button danger" disabled={busy} onClick={() => void stop()}>Disable inspection</button></div>
      <p className="hint">Configure your HTTP client to use the gateway URL. Requests sent directly to the backend bypass this inspector.</p>
      <div className="traffic-tools"><label>Method <select value={method} onChange={(event) => setMethod(event.target.value)}><option value="">All</option>{["GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"].map((value) => <option key={value}>{value}</option>)}</select></label><label>Status <select value={status} onChange={(event) => setStatus(event.target.value)}><option value="">All</option>{["2", "3", "4", "5"].map((value) => <option key={value} value={value}>{value}xx</option>)}</select></label><label>Search <input value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Path" /></label><button className="button" onClick={() => { pausedRef.current = !paused; setPaused(!paused); if (paused) syncRef.current(); }}>{paused ? "Resume" : "Pause"}</button><button className="button" onClick={() => void clear()}>Clear</button></div>
      <p className="traffic-state" role="status">{paused ? "Display paused; gateway still forwarding" : stream === "connected" ? "Live" : `Event stream ${stream.toLowerCase()}; checking history`}</p>
      <div className="traffic-list" role="list" aria-label="Captured requests">{visible.length ? visible.map((item) => <button role="listitem" className={`traffic-row ${item.id === selected ? "selected" : ""}`} key={item.id} onClick={() => setSelected(item.id)}><time>{new Date(item.started_at).toLocaleTimeString()}</time><strong>{item.method}</strong><code title={item.path}>{item.path}</code><span className={item.status_code >= 400 ? "traffic-error" : ""}>{item.status_code || "—"}</span><span>{item.duration_ms} ms</span>{item.outcome !== "completed" && <span className="traffic-error">{item.outcome}</span>}</button>) : <p className="traffic-empty">No matching requests yet. Send an HTTP request to the gateway URL.</p>}</div>
      {detail && <div className="traffic-detail"><h3>Request details</h3><dl><dt>Time</dt><dd>{new Date(detail.started_at).toLocaleString()}</dd><dt>Method</dt><dd>{detail.method}</dd><dt>Path</dt><dd>{detail.path}</dd><dt>Status</dt><dd>{detail.status_code || "—"}</dd><dt>Duration</dt><dd>{detail.duration_ms} ms</dd><dt>Request size</dt><dd>{detail.request_bytes ?? "Unknown"}</dd><dt>Response size</dt><dd>{detail.response_bytes}</dd><dt>Outcome</dt><dd>{detail.outcome}</dd>{detail.error && <><dt>Error</dt><dd>{detail.error}</dd></>}</dl></div>}
    </> : <div className="traffic-off"><label>HTTP port <select value={port} onChange={(event) => setPort(event.target.value)}>{ports.length ? ports.map((item) => <option key={item.key} value={item.key}>{item.label}</option>) : <option value="">No HTTP port available</option>}</select></label><button className="button primary" disabled={busy || !port} onClick={() => void start()}>Inspect HTTP traffic</button><p>Inspection creates a temporary loopback gateway. Your service keeps running when inspection stops.</p></div>}
    {error && <p className="error-banner" role="alert">{error}</p>}
  </section>;
}
