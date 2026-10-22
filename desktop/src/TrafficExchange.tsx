import { useEffect, useState } from "react";
import { Download } from "lucide-react";
import { api, type TrafficExchange as Exchange, type TrafficMessage, type TrafficPreview, type TrafficRecord } from "./api";

type Pair = [string, string];
const headerPairs = (headers: TrafficMessage["headers"]): Pair[] => Object.entries(headers ?? {}).flatMap(([name, values]) => values.map((value): Pair => [name, value]));
const header = (message: TrafficMessage, name: string) => headerPairs(message.headers).filter(([key]) => key.toLowerCase() === name).map(([, value]) => value);
const parameters = (text: string): Pair[] => Array.from(new URLSearchParams(text).entries());

function highlightJson(text: string) {
  const tokens = /"(?:\\.|[^"\\])*"|-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?|\b(?:true|false|null)\b/g;
  const parts = [];
  let end = 0;
  for (const match of text.matchAll(tokens)) {
    const start = match.index;
    parts.push(text.slice(end, start));
    let next = start + match[0].length;
    while (next < text.length && /\s/.test(text[next])) next++;
    const kind = match[0].startsWith('"') ? (text[next] === ":" ? "key" : "string") : /^(true|false|null)$/.test(match[0]) ? "literal" : "number";
    parts.push(<span className={`traffic-token-${kind}`} key={start}>{match[0]}</span>);
    end = start + match[0].length;
  }
  parts.push(text.slice(end));
  return parts;
}

function highlightHeader(line: string) {
  const colon = line.indexOf(":");
  return colon < 0 ? line : <><span className="traffic-token-key">{line.slice(0, colon)}</span><span className="traffic-token-punctuation">:</span>{line.slice(colon + 1)}</>;
}

function Pairs({ items, empty = "None" }: { items: Pair[]; empty?: string }) {
  return items.length ? <table className="traffic-values"><thead><tr><th scope="col">Name</th><th scope="col">Value</th></tr></thead><tbody>{items.map(([name, value], index) => <tr key={index}><th scope="row">{name || "(empty)"}</th><td><code>{value || "(empty)"}</code></td></tr>)}</tbody></table> : <p className="traffic-empty">{empty}</p>;
}

function MessagePane({ side, exchange, download }: { side: "request" | "response"; exchange: Exchange; download: () => Promise<string | null> }) {
  const [view, setView] = useState("Pretty");
  const [pretty, setPretty] = useState(true);
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState("");
  const message = exchange[side];
  const preview: TrafficPreview = exchange[`${side}_preview`];
  const title = side === "request" ? "Request" : "Response";
  const fields = headerPairs(message.headers);
  const trailers = headerPairs(message.trailers);
  const contentType = header(message, "content-type").join(";");
  const isJson = /json/i.test(contentType) && !preview.binary;
  const cookies: Pair[] = side === "request"
    ? header(message, "cookie").flatMap((value) => value.split(";")).map((value) => { const index = value.indexOf("="); return index < 0 ? [value.trim(), ""] : [value.slice(0, index).trim(), value.slice(index + 1)]; })
    : header(message, "set-cookie").map((value) => { const index = value.indexOf("="); return index < 0 ? [value, ""] : [value.slice(0, index), value.slice(index + 1)]; });
  const requestLine = `${exchange.method} ${exchange.path} ${message.protocol}`;
  const responseLine = `${message.protocol} ${message.status || exchange.status_code}`;
  const rawHeaders = fields.map(([name, value]) => `${name}: ${value}`);
  if (message.transfer_encoding?.length && !header(message, "transfer-encoding").length) rawHeaders.push(`Transfer-Encoding: ${message.transfer_encoding.join(", ")}`);
  let body = preview.text;
  if ((view === "Pretty" || (view === "Body" && pretty)) && isJson && !preview.truncated) {
    try { body = JSON.stringify(JSON.parse(body), null, 2); } catch { /* Preserve malformed JSON for inspection. */ }
  }
  const save = async () => {
    setSaving(true); setNotice("");
    try { if (await download()) setNotice("Body saved."); }
    catch (problem) { setNotice(`Download failed: ${String(problem)}`); }
    finally { setSaving(false); }
  };
  return <section className="traffic-detail-pane" aria-label={`${title} details`}>
    <div className="traffic-message-heading">
      <h4>{title}{side === "response" && <span className={exchange.status_code >= 400 ? "traffic-error" : "traffic-success"}>{exchange.status_code || "—"}</span>}</h4>
      <dl className="traffic-message-meta">{side === "request" ? <><dt>Time</dt><dd><time dateTime={exchange.started_at} title={new Date(exchange.started_at).toLocaleString()}>{new Date(exchange.started_at).toLocaleTimeString()}</time></dd><dt>Request size</dt><dd>{exchange.request_bytes == null ? "Unknown" : `${exchange.request_bytes} B`}</dd></> : <><dt>Duration</dt><dd>{exchange.duration_ms} ms</dd><dt>Response size</dt><dd>{exchange.response_bytes} B</dd><dt>Outcome</dt><dd>{exchange.outcome.replaceAll("_", " ")}</dd></>}</dl>
    </div>
    <div className="traffic-message-views" role="group" aria-label={`${title} views`}>{["Pretty", "Raw", "Headers", ...(side === "request" ? ["Query"] : []), "Cookies", "Body"].map((name) => <button key={name} className="button subtle" aria-pressed={view === name} onClick={() => setView(name)}>{name}</button>)}</div>
    <div className="traffic-message-content" tabIndex={0} role="region" aria-label={`${title} content`} key={view}>
      {side === "response" && exchange.error && <p className="traffic-error" role="status">{exchange.error}</p>}
      {!message.body.complete && <p className="traffic-capture-warning" role="status">Incomplete capture: {message.body.error || "The stream did not finish."} Retained {message.body.captured_bytes} of {message.body.bytes} observed bytes.</p>}
      {view === "Headers" && <><Pairs items={fields} empty="No headers captured" />{trailers.length > 0 && <><h5>Trailers</h5><Pairs items={trailers} /></>}</>}
      {view === "Query" && <><code className="traffic-query">{exchange.raw_query}</code><Pairs items={parameters(exchange.raw_query)} empty="No query parameters" /></>}
      {view === "Cookies" && <Pairs items={cookies} empty="No cookies" />}
      {(view === "Pretty" || view === "Raw" || view === "Body") && <>
        {preview.error && <p className="traffic-capture-warning">{preview.error}</p>}
        {preview.truncated && <p className="traffic-capture-warning">Preview shortened. Download the body for all {message.body.captured_bytes} retained bytes.</p>}
        {preview.binary && <p className="traffic-capture-note">Binary body · hexadecimal preview</p>}
        {view !== "Body" ? <pre aria-label={`${title} ${view.toLowerCase()} message`}><span className="traffic-token-line">{side === "request" ? requestLine : responseLine}</span>{rawHeaders.map((line, index) => <span key={index}>{"\n"}{highlightHeader(line)}</span>)}{"\n\n"}{isJson ? highlightJson(body) : body}{trailers.length ? <>{"\n\nTrailers:\n"}{trailers.map(([name, value], index) => <span key={index}>{index > 0 ? "\n" : ""}{highlightHeader(`${name}: ${value}`)}</span>)}</> : null}</pre> : <>
          {isJson && <label><input type="checkbox" checked={pretty} onChange={(event) => setPretty(event.target.checked)} />Format JSON</label>}
          {contentType.includes("application/x-www-form-urlencoded") && <Pairs items={parameters(preview.text)} />}
          {preview.fields?.length ? <><h5>Multipart fields</h5><Pairs items={preview.fields.map((field) => [field.name, field.filename ? `${field.filename} (${field.content_type || "file"}) — ${field.value}` : field.value])} /></> : null}
          <pre aria-label={`${title} body`}>{isJson && body ? highlightJson(body) : body || (preview.error ? "Preview unavailable" : "(empty body)")}</pre>
        </>}
      </>}
    </div>
    <div className="traffic-body-download"><span>{message.body.captured_bytes} B retained</span><button className="button subtle" aria-label={`Download ${side} body`} disabled={saving} onClick={() => void save()}><Download size={14} aria-hidden="true" />{saving ? "Saving…" : "Download body"}</button></div>
    {notice && <p role="status" className="traffic-capture-note">{notice}</p>}
  </section>;
}

export default function TrafficExchange({ workspaceId, sessionId, record }: { workspaceId: string; sessionId: string; record: TrafficRecord }) {
  const [exchange, setExchange] = useState<Exchange | null>(null);
  const [error, setError] = useState("");
  useEffect(() => {
    let active = true;
    setExchange(null); setError("");
    api.trafficExchange(workspaceId, sessionId, record.id).then((value) => { if (active) setExchange(value); }).catch((problem) => { if (active) setError(String(problem)); });
    return () => { active = false; };
  }, [workspaceId, sessionId, record.id]);
  if (error) return <p className="error-banner" role="alert">{error}</p>;
  if (!exchange) return <p className="traffic-empty" role="status">Loading captured exchange…</p>;
  return <div className="traffic-detail-panes">{(["request", "response"] as const).map((side) => <MessagePane key={`${exchange.id}:${side}`} side={side} exchange={exchange} download={() => api.downloadTrafficBody(workspaceId, sessionId, exchange.id, side)} />)}</div>;
}
