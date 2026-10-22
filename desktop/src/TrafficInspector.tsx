import { useCallback, useEffect, useRef, useState } from "react";
import { listen } from "@tauri-apps/api/event";
import { ArrowRight, Circle, Copy, Pause, Play, Search, Square, Trash2 } from "lucide-react";
import { ApiError, api, type TrafficRecord, type TrafficSession } from "./api";
import TrafficExchange from "./TrafficExchange";

type Port = { key: string; label: string };
type Event = { type: string; workspace_id: string; data?: { session_id?: string } };

const bytes = (value?: number) => value == null ? "Unknown" : `${value} B`;

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
      setSessions((current) => current.map((item) => item.id === session.id ? { ...item, state: "stopped" } : item));
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
  const detail = visible.find((item) => item.id === selected);
  return <section className="traffic-inspector" aria-label="HTTP traffic inspector">
    <div className="traffic-heading">
      <h2>Traffic inspector</h2>
      <div className="traffic-heading-actions">
        <span className={`traffic-session-state ${session?.state === "active" ? "traffic-success" : ""}`}><Circle size={9} fill="currentColor" aria-hidden="true" />{session?.state ?? "Off"}</span>
        {session?.state === "active" && <button className="button subtle" disabled={busy} onClick={() => void stop()}><Square size={14} aria-hidden="true" />Disable inspection</button>}
      </div>
    </div>
    {(!session || session.state === "stopped") && <div className="traffic-off"><label>HTTP port <select value={port} onChange={(event) => setPort(event.target.value)}>{ports.length ? ports.map((item) => <option key={item.key} value={item.key}>{item.label}</option>) : <option value="">No HTTP port available</option>}</select></label><button className="button primary" disabled={busy || !port} onClick={() => void start()}>Inspect HTTP traffic</button><p>Capture includes headers, credentials, cookies, and bodies. History stays on this device for 7 days, up to 500 exchanges per session and 512 MiB total.</p></div>}
    {session && <>
      {sessions.length > 1 && <label>Session <select value={sessionId} onChange={(event) => setSessionId(event.target.value)}>{sessions.map((item) => <option key={item.id} value={item.id}>{item.target.port_key} · {item.state} · {new Date(item.started_at).toLocaleString()}</option>)}</select></label>}
      {session.state === "active" ? <div className="traffic-target">
        <div className="traffic-endpoint"><span>Gateway</span><div className="traffic-address"><code>{session.proxy_url}</code><button className="button subtle" onClick={() => navigator.clipboard.writeText(session.proxy_url).catch((problem) => setError(String(problem)))}><Copy size={15} aria-hidden="true" />Copy URL</button></div></div>
        <ArrowRight className="traffic-direction" size={20} aria-hidden="true" />
        <div className="traffic-endpoint"><span>Backend</span><div className="traffic-address"><code>{session.upstream}</code></div></div>
        <p>Send requests to the gateway to capture traffic.</p>
      </div> : <p className="traffic-capture-note">Inspection stopped. Saved history remains available; the gateway URL is no longer listening.</p>}
      <div className="traffic-history">
        <div className="traffic-tools">
          <label className="traffic-search"><Search size={17} aria-hidden="true" /><input aria-label="Search paths" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search paths" /></label>
          <label>Method <select value={method} onChange={(event) => setMethod(event.target.value)}><option value="">All</option>{["GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS", "CONNECT", "TRACE"].map((value) => <option key={value}>{value}</option>)}</select></label>
          <label>Status <select value={status} onChange={(event) => setStatus(event.target.value)}><option value="">All</option>{["1", "2", "3", "4", "5"].map((value) => <option key={value} value={value}>{value}xx</option>)}</select></label>
          <div className="traffic-tool-actions">
            <button className="button subtle" onClick={() => { pausedRef.current = !paused; setPaused(!paused); if (paused) syncRef.current(); }}>{paused ? <Play size={14} aria-hidden="true" /> : <Pause size={14} aria-hidden="true" />}{paused ? "Resume" : "Pause"}</button>
            <button className="button subtle" onClick={() => void clear()}><Trash2 size={15} aria-hidden="true" />Clear</button>
          </div>
        </div>
        <div className="traffic-history-heading">
          <h3>History</h3><span>{visible.length} of {records.length} requests</span>
          <span className={`traffic-state ${paused ? "traffic-warning" : stream === "connected" ? "traffic-success" : ""}`} role="status"><Circle size={8} fill="currentColor" aria-hidden="true" />{session.state === "stopped" ? "Saved history" : paused ? "Display paused; gateway still forwarding" : stream === "connected" ? "Live" : `Event stream ${stream.toLowerCase()}; checking history`}</span>
        </div>
        <div className="traffic-list" tabIndex={0} role="region" aria-label="Captured request history">
          <table className="traffic-table" aria-label="Captured requests">
            <colgroup><col className="traffic-col-sequence" /><col className="traffic-col-time" /><col className="traffic-col-method" /><col /><col className="traffic-col-status" /><col className="traffic-col-metric" /><col className="traffic-col-metric" /><col className="traffic-col-metric" /></colgroup>
            <thead><tr>{["#", "Time", "Method", "Path", "Status", "Duration", "Request", "Response"].map((label) => <th scope="col" key={label}>{label}</th>)}</tr></thead>
            <tbody>{visible.map((item) => <tr className={`traffic-row ${item.id === selected ? "selected" : ""}`} key={item.id} onClick={() => setSelected((current) => current === item.id ? "" : item.id)}>
              <td>{item.sequence}</td><td><time dateTime={item.started_at}>{new Date(item.started_at).toLocaleTimeString()}</time></td><td>{item.method}</td>
              <td><button className="traffic-path" aria-label={`Inspect request #${item.sequence}: ${item.method} ${item.path}`} aria-pressed={item.id === selected} title={item.path}><code>{item.path}</code></button></td>
              <td className={item.status_code >= 400 || item.outcome !== "completed" ? "traffic-error" : item.status_code >= 200 && item.status_code < 300 ? "traffic-success" : ""} title={item.outcome.replaceAll("_", " ")}>{item.status_code || "—"}</td>
              <td>{item.duration_ms} ms{item.capture_incomplete && <span className="traffic-incomplete" title={item.capture_error || "Some content was not fully captured"}>Incomplete</span>}</td><td>{bytes(item.request_bytes)}</td><td>{bytes(item.response_bytes)}</td>
            </tr>)}</tbody>
          </table>
          {!visible.length && <p className="traffic-empty">{records.length ? "No requests match these filters. Adjust your search, method, or status." : "No requests captured yet. Send an HTTP request to the gateway URL."}</p>}
        </div>
      </div>
      <div className="traffic-detail" hidden={!detail}>
        <div className="traffic-detail-heading"><h3>Selected exchange</h3>{detail && <span>#{detail.sequence}</span>}<span className="traffic-resize-hint">Drag corner to resize</span><button className="button subtle" onClick={() => setSelected("")}>Hide</button></div>
        {detail?.capture_error && <p className="error-banner" role="alert">{detail.capture_error}</p>}
        {detail && <TrafficExchange key={`${sessionId}:${detail.id}`} workspaceId={workspaceId} sessionId={sessionId} record={detail} />}
        <details className="traffic-capture-info"><summary>Capture information</summary><p>Messages are reconstructed. Compressed bodies are decoded for viewing; header order and chunk framing are not retained.</p><p>Content is retained on this device until cleared or expired. Previews are limited to 256 KiB; downloads include all retained bytes. Clearing history also discards captures currently in flight when they finish.</p></details>
      </div>
    </>}
    {error && <p className="error-banner" role="alert">{error}</p>}
  </section>;
}
