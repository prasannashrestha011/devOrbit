import { useCallback, useEffect, useRef, useState } from "react";
import TerminalPane from "./TerminalPane";
import DockerPanel from "./DockerPanel";
import YamlSource from "./YamlSource";
import EnvironmentEditor, { type EnvDraft } from "./EnvironmentEditor";
import TrafficInspector from "./TrafficInspector";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { homeDir } from "@tauri-apps/api/path";
import { open } from "@tauri-apps/plugin-dialog";
import {
  Activity,
  ChevronLeft,
  ChevronRight,
  FileCode2,
  Folder,
  GitBranch as GitBranchIcon,
  LayoutGrid,
  List,
  Pencil,
  Play,
  Plus,
  RotateCw,
  Search,
  Settings,
  Square,
  Terminal,
  Trash2,
} from "lucide-react";
import { formatCommandLine, parseCommandLine } from "./commandLine";
import {
  api,
  ApiError,
  type LogEntry,
  type LogPage,
  type OperationResult,
  type Service,
  type ServiceInput,
	type WorkspaceConfiguration,
  type ShellSession,
  type Workspace,
  type WorkspaceInput,
  type WorkspaceStatus,
} from "./api";

type Dialog =
  | { kind: "workspace"; item?: Workspace }
  | { kind: "service"; item?: Service }
	| { kind: "configuration" }
  | null;
type Notice = { workspace_id: string; type?: string };

const icons = {
  grid: LayoutGrid,
  folder: Folder,
  plus: Plus,
  play: Play,
  stop: Square,
  restart: RotateCw,
  logs: List,
  file: FileCode2,
  settings: Settings,
  edit: Pencil,
  trash: Trash2,
  arrow: ChevronRight,
  back: ChevronLeft,
  search: Search,
  pulse: Activity,
  terminal: Terminal,
};

function Icon({ name, size = 18 }: { name: keyof typeof icons; size?: number }) {
  const IconComponent = icons[name];
  return <IconComponent size={size} strokeWidth={1.8} aria-hidden="true" />;
}

function GitBranch({ branch }: { branch: string }) {
  return branch ? <span className="git-branch" title={branch}><GitBranchIcon size={14} strokeWidth={1.8} aria-hidden="true" /><span>{branch}</span></span> : null;
}

function StateBadge({ state }: { state: string }) {
  return (
    <span className={`state state-${state}`}>
      <span className="state-dot" />
      {state.replace("_", " ")}
    </span>
  );
}

function errorMessage(error: unknown) {
  if (error instanceof ApiError) {
    if (error.status === 401 || error.status === 403)
      return "Authentication failed. Check that the desktop and backend use the same token path.";
    if (error.code === "unavailable") return error.message;
    return error.message;
  }
  return String(error);
}

async function browseDirectory(setPath: (path: string) => void, setError: (error: string) => void) {
  try {
    const defaultPath = localStorage.getItem("devorbit.lastPickerDirectory") || await homeDir();
    const path = await open({ directory: true, multiple: false, defaultPath });
    if (path) {
      localStorage.setItem("devorbit.lastPickerDirectory", path);
      setPath(path);
    }
  } catch (error) {
    setError(errorMessage(error));
  }
}

function relativeDirectory(workspaceRoot: string, path: string): string | null {
  const root = workspaceRoot.replace(/\\/g, "/").replace(/\/+$/, "") || "/";
  const chosen = path.replace(/\\/g, "/").replace(/\/+$/, "") || "/";
  const prefix = root === "/" ? root : `${root}/`;
  const windows = /^[A-Za-z]:\//.test(root);
  const matchRoot = windows ? root.toLowerCase() : root;
  const matchChosen = windows ? chosen.toLowerCase() : chosen;
  if (matchChosen === matchRoot) return "";
  return matchChosen.startsWith(windows ? prefix.toLowerCase() : prefix) ? chosen.slice(prefix.length) : null;
}

function WorkspaceForm({
  item,
  onSave,
  onClose,
}: {
  item?: Workspace;
  onSave: (value: WorkspaceInput) => Promise<void>;
  onClose: () => void;
}) {
  const [name, setName] = useState(item?.name ?? "");
  const [description, setDescription] = useState(item?.description ?? "");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  return (
    <div
      className="dialog-backdrop"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
      onKeyDown={(event) => {
        if (event.key === "Escape") onClose();
      }}
    >
      <form
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="workspace-dialog-title"
        onSubmit={async (event) => {
          event.preventDefault();
          setSaving(true);
          setError("");
          try {
            await onSave({
              name: name.trim(),
              description,
            });
          } catch (error) {
            setError(errorMessage(error));
            setSaving(false);
          }
        }}
      >
        <div className="dialog-title">
          <div>
            <span className="eyebrow">WORKSPACE</span>
            <h2 id="workspace-dialog-title">
              {item ? "Edit workspace" : "New workspace"}
            </h2>
          </div>
          <button
            type="button"
            className="icon-button"
            onClick={onClose}
            aria-label="Close"
          >
            ×
          </button>
        </div>
        <label>
          Name
          <input
            required
            autoFocus
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="My project"
          />
        </label>
        <label>
          Description
          <textarea
            rows={3}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="What runs in this workspace?"
          />
        </label>
        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}
        <div className="dialog-actions">
          <button type="button" className="button subtle" onClick={onClose}>
            Cancel
          </button>
          <button className="button primary" disabled={saving}>
            {saving ? "Saving…" : item ? "Save changes" : "Create workspace"}
          </button>
        </div>
      </form>
    </div>
  );
}

function ServiceForm({
  item,
  workspaceRoot,
  onSave,
  onClose,
}: {
  item?: Service;
  workspaceRoot: string;
  onSave: (value: Omit<ServiceInput, "revision">) => Promise<void>;
  onClose: () => void;
}) {
  const [name, setName] = useState(item?.name ?? "");
  const [commandLine, setCommandLine] = useState(
    item ? formatCommandLine([item.command, ...item.args]) : "",
  );
  const [directory, setDirectory] = useState(item?.working_directory ?? "");
  const [ports, setPorts] = useState(
    Object.entries(item?.ports ?? {})
      .map(([k, v]) => `${k}=${v}`)
      .join("\n"),
  );
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  return (
    <div
      className="dialog-backdrop"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
      onKeyDown={(event) => {
        if (event.key === "Escape") onClose();
      }}
    >
      <form
        className="dialog service-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="service-dialog-title"
        onSubmit={async (event) => {
          event.preventDefault();
          setError("");
          const namedPorts: Record<string, number> = {};
          for (const line of ports.split("\n").filter((line) => line.trim())) {
            const at = line.indexOf("=");
            const port = Number(line.slice(at + 1));
            if (at < 1 || !Number.isInteger(port) || port < 1 || port > 65535) {
              setError("Use name=port (1–65535) for each port.");
              return;
            }
            namedPorts[line.slice(0, at)] = port;
          }
          setSaving(true);
          try {
            const [command, ...args] = parseCommandLine(commandLine);
            const enteredDirectory = directory.trim();
            const workingDirectory = enteredDirectory.startsWith("/") || /^[A-Za-z]:[\\/]/.test(enteredDirectory)
              ? relativeDirectory(workspaceRoot, enteredDirectory) ?? enteredDirectory
              : enteredDirectory;
            await onSave({
              name: name.trim(),
              command,
              args,
              working_directory: workingDirectory,
              ports: namedPorts,
            });
          } catch (error) {
            setError(errorMessage(error));
            setSaving(false);
          }
        }}
      >
        <div className="dialog-title">
          <div>
            <span className="eyebrow">SERVICE CONFIGURATION</span>
            <h2 id="service-dialog-title">
              {item ? "Edit service" : "Add a service"}
            </h2>
          </div>
          <button
            type="button"
            className="icon-button"
            onClick={onClose}
            aria-label="Close"
          >
            ×
          </button>
        </div>
        {item && (
          <p className="hint">
            Saving configuration does not restart a running process. Use Restart
            when ready.
          </p>
        )}
        <label>
          Name
          <input
            required
            autoFocus
            disabled={!!item}
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="API server"
          />
        </label>
        <div className="path-control">
          <label htmlFor="service-command">Command</label>
          <input
            id="service-command"
            aria-describedby="command-hint"
            required
            value={commandLine}
            onChange={(e) => setCommandLine(e.target.value)}
            placeholder="flutter run -d linux"
          />
          <small id="command-hint">Use quotes for arguments with spaces. Shell operators are not supported.</small>
        </div>
        <div className="path-control">
          <label htmlFor="service-directory">Working directory</label>
          <div className="path-field">
            <input id="service-directory" value={directory} onChange={(e) => setDirectory(e.target.value)} placeholder="Workspace root" />
            <button
              type="button"
              className="button subtle"
              aria-label="Browse for service working directory"
              onClick={() => browseDirectory((path) => {
                setDirectory(path);
                setError("");
              }, setError)}
            >
              Browse…
            </button>
          </div>
          <small>Folders inside the workspace are saved as relative paths. Other folders are saved as absolute paths.</small>
        </div>
        <label>
          Expected ports <span className="label-note">one name=port per line</span>
          <textarea
            className="mono"
            rows={3}
            value={ports}
            onChange={(e) => setPorts(e.target.value)}
            placeholder="http=8080"
          />
          <small>Ports are informational; DevOrbit does not bind or inject them.</small>
        </label>
        {error && (
          <p className="form-error" role="alert">
            {error}
          </p>
        )}
        <div className="dialog-actions">
          <button type="button" className="button subtle" onClick={onClose}>
            Cancel
          </button>
          <button className="button primary" disabled={saving}>
            {saving ? "Saving…" : item ? "Save changes" : "Add service"}
          </button>
        </div>
      </form>
    </div>
  );
}

function ConfigurationEditor({ workspaceId, initial, onSaved, onClose }: {
  workspaceId: string;
  initial: WorkspaceConfiguration;
  onSaved: () => Promise<void>;
  onClose: () => void;
}) {
  const [content, setContent] = useState(initial.content);
  const [revision, setRevision] = useState(initial.revision);
  const [preview, setPreview] = useState<string[] | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  async function validate() {
    setError("");
    try {
      const result = await api.validateConfiguration(workspaceId, content);
      setPreview(Object.keys(result.services));
      return true;
    } catch (problem) {
      setPreview(null);
      setError(errorMessage(problem));
      return false;
    }
  }
  return <div className="dialog-backdrop" onMouseDown={(event) => { if (event.target === event.currentTarget) onClose(); }}>
    <div className="dialog configuration-dialog" role="dialog" aria-modal="true" aria-labelledby="configuration-dialog-title">
      <div className="dialog-title"><div><span className="eyebrow">WORKSPACE CONFIGURATION</span><h2 id="configuration-dialog-title">.devorbit/workspace.yaml</h2></div><button className="icon-button" onClick={onClose} aria-label="Close">×</button></div>
      <p className="hint">Save updates service definitions. Running processes keep their current configuration until you restart them.</p>
      <label htmlFor="configuration-file">Import YAML into editor</label>
      <input id="configuration-file" type="file" accept=".yaml,.yml,text/yaml" onChange={async (event) => {
        const file = event.target.files?.[0];
        if (!file) return;
        setContent(await file.text());
        setPreview(null);
      }} />
      <label htmlFor="configuration-source">YAML</label>
      <YamlSource value={content} onChange={(value) => { setContent(value); setPreview(null); }} />
      {preview && <p className="hint" role="status">Valid: {preview.length} service{preview.length === 1 ? "" : "s"}{preview.length ? ` — ${preview.join(", ")}` : ""}</p>}
      {initial.revision !== revision && <p className="form-error" role="alert">The YAML file changed outside this editor. Reload it before saving or your save will be rejected.</p>}
      {error && <p className="form-error" role="alert">{error}</p>}
      <div className="dialog-actions">
        <button className="button subtle" disabled={busy} onClick={async () => { try { const latest = await api.configuration(workspaceId); setContent(latest.content); setRevision(latest.revision); setPreview(null); setError(""); } catch (problem) { setError(errorMessage(problem)); } }}>Reload file</button>
        <button className="button subtle" disabled={busy} onClick={() => void validate()}>Validate</button>
        <button className="button primary" disabled={busy} onClick={async () => {
          setBusy(true);
          try {
            if (!await validate()) return;
            await api.saveConfiguration(workspaceId, content, revision);
            await onSaved();
          } catch (problem) { setError(errorMessage(problem)); }
          finally { setBusy(false); }
        }}>Save YAML</button>
      </div>
    </div>
  </div>;
}

function LogViewer({
  page,
  entries,
  loading,
  streamState,
  onOlder,
}: {
  page: LogPage | null;
  entries: LogEntry[];
  loading: boolean;
  streamState: string;
  onOlder: () => void;
}) {
  const [filter, setFilter] = useState<"all" | "stdout" | "stderr" | "terminal">("all");
  const shown = entries
    .filter((entry) => filter === "all" || entry.stream === filter)
    .slice()
    .reverse();
  return (
    <section className="log-panel">
      <div className="log-toolbar">
        <div className="panel-heading">
          <Icon name="terminal" size={17} />
          <strong>Process output</strong>
          <span className="muted">persisted history</span>
        </div>
        <div className="log-tools">
          <select
            aria-label="Log stream"
            value={filter}
            onChange={(e) => setFilter(e.target.value as typeof filter)}
          >
            <option value="all">All streams</option>
            <option value="stdout">stdout</option>
            <option value="stderr">stderr</option>
            <option value="terminal">terminal</option>
          </select>
          <span
            className={`live-mark ${streamState === "connected" ? "online" : ""}`}
          >
            <span className="state-dot" />{" "}
            {streamState === "connected"
              ? "Live updates"
              : streamState === "authentication-error"
                ? "Authentication error"
                : "Reconnecting"}
          </span>
        </div>
      </div>
      {page &&
        (page.diagnostics.dropped_entries > 0 ||
          page.diagnostics.storage_errors > 0) && (
          <div className="diagnostic" role="status">
            Some output could not be saved: {page.diagnostics.dropped_entries}{" "}
            dropped, {page.diagnostics.storage_errors} storage errors.
          </div>
        )}
      <div className="log-content">
        {shown.length === 0 ? (
          <div className="log-empty">
            {loading
              ? "Loading logs…"
              : "No output yet. Logs appear here after the service starts."}
          </div>
        ) : (
          shown.map((entry) => (
            <div className={`log-line ${entry.stream}`} key={entry.id}>
              <time title={new Date(entry.timestamp).toLocaleString()}>
                {new Date(entry.timestamp).toLocaleTimeString()}
              </time>
              <span className="stream-label">{entry.stream}</span>
              <span className="log-message">{entry.message}</span>
            </div>
          ))
        )}
      </div>
      <div className="log-footer">
        <span>Newest {entries.length} entries</span>
        {page?.entries.length === 100 && (
          <button className="text-button" onClick={onOlder} disabled={loading}>
            {loading ? "Loading…" : "Load older logs"}
          </button>
        )}
      </div>
    </section>
  );
}

export default function App() {
  const [workspaces, setWorkspaces] = useState<Workspace[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [selectedServiceId, setSelectedServiceId] = useState<string | null>(
    null,
  );
  const [workspaceView, setWorkspaceView] = useState<"overview" | "terminals">("overview");
  const [shellSessions, setShellSessions] = useState<ShellSession[]>([]);
  const [activeTerminalId, setActiveTerminalId] = useState<string | null>(null);
  const [terminalFonts, setTerminalFonts] = useState<string[]>([]);
  const [terminalFont, setTerminalFont] = useState(() => localStorage.getItem("devorbit.terminalFont") ?? "");
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [services, setServices] = useState<Service[]>([]);
	const [configuration, setConfiguration] = useState<WorkspaceConfiguration | null>(null);
  const [status, setStatus] = useState<WorkspaceStatus | null>(null);
  const [page, setPage] = useState<LogPage | null>(null);
  const [entries, setEntries] = useState<LogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [workspaceLoading, setWorkspaceLoading] = useState(false);
  const [logsLoading, setLogsLoading] = useState(false);
  const [error, setError] = useState("");
  const [streamState, setStreamState] = useState("connecting");
  const [dialog, setDialog] = useState<Dialog>(null);
  const [busy, setBusy] = useState("");
  const [result, setResult] = useState<OperationResult | null>(null);
  const [search, setSearch] = useState("");
  const [serviceDetailTab, setServiceDetailTab] = useState<"logs" | "environment" | "traffic">("logs");
  const [envDrafts, setEnvDrafts] = useState<Record<string, EnvDraft>>({});
  const [envSelections, setEnvSelections] = useState<Record<string, string>>({});
  const selectedRef = useRef<string | null>(null);
  const serviceRef = useRef<string | null>(null);
  const refreshTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const configurationRef = useRef<WorkspaceConfiguration | null>(null);
  selectedRef.current = selectedId;
  serviceRef.current = selectedServiceId;
  configurationRef.current = configuration;

  const loadWorkspaces = useCallback(async () => {
    const list = await api.workspaces();
    setWorkspaces(list);
    setSelectedId((current) =>
      current && list.some((item) => item.id === current)
        ? current
        : (list[0]?.id ?? null),
    );
    return list;
  }, []);

  const loadWorkspace = useCallback(async (id: string) => {
    const [config, shells] = await Promise.all([api.configuration(id), api.shellTerminals(id)]);
    const [itemsResult, statusResult] = await Promise.allSettled([api.services(id), api.workspaceStatus(id)]);
    if (selectedRef.current === id) {
      setConfiguration(config);
      setServices(itemsResult.status === "fulfilled" ? itemsResult.value : []);
      setStatus(statusResult.status === "fulfilled" ? statusResult.value : null);
      setShellSessions(shells);
      if (itemsResult.status === "rejected") setError(errorMessage(itemsResult.reason));
    }
  }, []);

  const loadShells = useCallback(async (id: string) => {
    const shells = await api.shellTerminals(id);
    if (selectedRef.current === id) {
      setShellSessions(shells);
    }
  }, []);

  const loadLogs = useCallback(async (id: string) => {
    setLogsLoading(true);
    try {
      const next = await api.logs(id);
      if (serviceRef.current === id) {
        setPage(next);
        setEntries(next.entries);
      }
    } finally {
      setLogsLoading(false);
    }
  }, []);

  useEffect(() => {
    loadWorkspaces()
      .catch((error) => setError(errorMessage(error)))
      .finally(() => setLoading(false));
  }, [loadWorkspaces]);

  useEffect(() => {
    invoke<string[]>("list_system_fonts")
      .then(setTerminalFonts)
      .catch((error) => setError(errorMessage(error)));
  }, []);

  useEffect(() => {
    setSelectedServiceId(null);
    setWorkspaceView("overview");
    setShellSessions([]);
    setActiveTerminalId(null);
    setServices([]);
    setConfiguration(null);
    setStatus(null);
    setResult(null);
    setSearch("");
    if (!selectedId) return;
    setWorkspaceLoading(true);
    setError("");
    loadWorkspace(selectedId)
      .catch((error) => setError(errorMessage(error)))
      .finally(() => setWorkspaceLoading(false));
  }, [selectedId, loadWorkspace]);

  useEffect(() => {
    setPage(null);
    setEntries([]);
    setServiceDetailTab("logs");
    if (selectedServiceId)
      loadLogs(selectedServiceId).catch((error) =>
        setError(errorMessage(error)),
      );
  }, [selectedServiceId, loadLogs]);

  useEffect(() => {
    if (!selectedId) return;
    const timer = setInterval(() => {
      Promise.all([api.configuration(selectedId), api.workspaceStatus(selectedId)]).then(([latest, current]) => {
        if (selectedRef.current !== selectedId) return;
        setStatus(current);
        if (latest.revision !== configurationRef.current?.revision) return loadWorkspace(selectedId);
      }).catch((problem) => setError(errorMessage(problem)));
    }, 4000);
    return () => clearInterval(timer);
  }, [selectedId, loadWorkspace]);

  useEffect(() => {
    let disposed = false;
    const subscriptions = Promise.all([
      listen<string>("stream-state", (event) => {
        if (disposed) return;
        setStreamState(event.payload);
        if (event.payload === "connected") {
          loadWorkspaces()
            .then(() => {
              if (selectedRef.current)
                return loadWorkspace(selectedRef.current);
            })
            .then(() => {
              if (serviceRef.current) return loadLogs(serviceRef.current);
            })
            .catch((error) => setError(errorMessage(error)));
        }
      }),
      listen<Notice>("backend-event", (event) => {
        if (disposed || event.payload.workspace_id !== selectedRef.current || event.payload.type?.startsWith("traffic."))
          return;
        if (refreshTimer.current) return;
        refreshTimer.current = setTimeout(() => {
          refreshTimer.current = null;
          if (selectedRef.current)
            loadWorkspace(selectedRef.current).catch((error) =>
              setError(errorMessage(error)),
            );
          if (serviceRef.current)
            loadLogs(serviceRef.current).catch((error) =>
              setError(errorMessage(error)),
            );
        }, 250);
      }),
    ]);
    subscriptions.then(() => {
      if (!disposed)
        invoke("start_events").catch((error) => setError(errorMessage(error)));
    });
    return () => {
      disposed = true;
      subscriptions.then((items) => items.forEach((unlisten) => unlisten()));
      invoke("stop_events").catch(() => {});
      if (refreshTimer.current) clearTimeout(refreshTimer.current);
    };
  }, [loadWorkspaces, loadWorkspace, loadLogs]);

  const workspace = workspaces.find((item) => item.id === selectedId);
  const selectedService = services.find(
    (item) => item.id === selectedServiceId,
  );
  const selectedStatus = status?.services.find(
    (item) => item.service_id === selectedServiceId,
  );
  const focusedTerminalId = services.some((item) => item.id === activeTerminalId) || shellSessions.some((item) => item.id === activeTerminalId)
    ? activeTerminalId
    : (services[0]?.id ?? shellSessions[0]?.id ?? null);
  const activeTerminalService = services.find((item) => item.id === focusedTerminalId);
  const activeTerminalStatus = status?.services.find((item) => item.service_id === focusedTerminalId);
  const activeShell = shellSessions.find((item) => item.id === focusedTerminalId);
  const filteredServices = services.filter((item) =>
    item.name.toLowerCase().includes(search.toLowerCase()),
  );

  async function runAction(key: string, action: () => Promise<void>) {
    setBusy(key);
    setError("");
    try {
      await action();
    } catch (error) {
      setError(errorMessage(error));
    } finally {
      setBusy("");
      if (selectedRef.current)
        loadWorkspace(selectedRef.current).catch((error) =>
          setError(errorMessage(error)),
        );
    }
  }

  function workspaceAction(action: "start" | "stop" | "restart") {
    if (!selectedId) return;
    runAction(`workspace-${action}`, async () => {
      if (action === "restart") {
        const stopped = await api.workspaceAction(selectedId, "stop");
        if (stopped.failed > 0) {
          setResult(stopped);
          throw new Error("Some services could not be stopped. Restart all was cancelled.");
        }
      }
      const outcome = await api.workspaceAction(selectedId, action === "restart" ? "start" : action);
      setResult(action === "restart" ? { ...outcome, operation: "restart" } : outcome);
      if (action === "start") {
        const first = outcome.results.find((item) => item.outcome === "started" || item.outcome === "already_running");
        if (first) {
          setSelectedServiceId(null);
          setActiveTerminalId(first.service_id);
          setWorkspaceView("terminals");
        }
      }
    });
  }

  function serviceAction(id: string, action: "start" | "stop" | "restart") {
    runAction(`${id}-${action}`, async () => {
      await api.serviceAction(id, action);
      if (action !== "stop") {
        setSelectedServiceId(null);
        setActiveTerminalId(id);
        setWorkspaceView("terminals");
      }
      if (serviceRef.current === id) await loadLogs(id);
    });
  }

  function createShellTerminal() {
    if (!selectedId) return;
    runAction("create-shell", async () => {
      const session = await api.createShellTerminal(selectedId);
      setShellSessions((current) => [...current, session]);
      setActiveTerminalId(session.id);
    });
  }

  function closeShellTerminal(id: string) {
    if (!selectedId) return;
    runAction(`close-shell-${id}`, async () => {
      await api.deleteShellTerminal(selectedId, id);
      const remaining = shellSessions.filter((item) => item.id !== id);
      setShellSessions(remaining);
      setActiveTerminalId((current) => current === id ? null : current);
    });
  }

  async function deleteService(item: Service) {
    if (
      !window.confirm(
        `Delete “${item.name}” from workspace.yaml? Stop it first if it is running.`,
      )
    )
      return;
    await runAction(`delete-${item.id}`, async () => {
      if (!configuration) throw new Error("Reload the workspace configuration first.");
      await api.deleteService(item.id, configuration.revision);
      if (selectedServiceId === item.id) setSelectedServiceId(null);
      await loadWorkspace(item.workspace_id);
    });
  }

  async function deleteWorkspace(item: Workspace) {
    if (!window.confirm(`Delete “${item.name}” and its services? Services and shell terminals will be stopped. Its workspace configuration will be deleted.`)) return;
    await runAction("delete-workspace", async () => {
      const stopped = await api.workspaceAction(item.id, "stop");
      if (stopped.failed) throw new Error("Stop all services before deleting the workspace.");
      for (const shell of await api.shellTerminals(item.id)) {
        await api.deleteShellTerminal(item.id, shell.id);
      }
      await api.deleteWorkspace(item.id);
      selectedRef.current = null;
      setSelectedId(null);
      await loadWorkspaces();
    });
  }

  async function saveWorkspace(input: WorkspaceInput) {
    const saved =
      dialog?.kind === "workspace" && dialog.item
        ? await api.updateWorkspace(dialog.item.id, input)
        : await api.createWorkspace(input);
    await loadWorkspaces();
    setSelectedId(saved.id);
    setDialog(null);
  }

  async function saveService(input: Omit<ServiceInput, "revision">) {
    if (!selectedId) return;
    if (!configuration) throw new Error("Reload the workspace configuration first.");
    const body = { ...input, revision: configuration.revision };
    const saved =
      dialog?.kind === "service" && dialog.item
        ? await api.updateService(dialog.item.id, body)
        : await api.createService(selectedId, body);
    await loadWorkspace(selectedId);
    setSelectedServiceId(saved.id);
    setDialog(null);
  }

  async function olderLogs() {
    if (!selectedServiceId || !page?.next_cursor) return;
    setLogsLoading(true);
    try {
      const next = await api.logs(selectedServiceId, page.next_cursor);
      setEntries((current) => [
        ...current,
        ...next.entries.filter(
          (entry) => !current.some((item) => item.id === entry.id),
        ),
      ]);
      setPage(next);
    } catch (error) {
      setError(errorMessage(error));
    } finally {
      setLogsLoading(false);
    }
  }

  return (
    <>
      <div className={`app-shell ${workspaceView === "terminals" && workspace ? "terminals-view" : ""}`} inert={!!dialog}>
        <aside className="sidebar">
          <div className="brand">
            <span className="brand-icon">
              <span />
            </span>
            <span>
              Dev<span className="brand-light">Orbit</span>
            </span>
          </div>
          <div className="side-section-title">
            YOUR WORKSPACES{" "}
            <button
              className="icon-button"
              onClick={() => setDialog({ kind: "workspace" })}
              title="Create workspace"
              aria-label="Create workspace"
            >
              <Icon name="plus" size={16} />
            </button>
          </div>
          <nav aria-label="Workspaces" className="workspace-nav">
            {workspaces.map((item) => (
              <button
                key={item.id}
                className={`workspace-nav-item ${item.id === selectedId ? "active" : ""}`}
                onClick={() => setSelectedId(item.id)}
                title={item.name}
              >
                <Icon name="folder" size={18} />
                <span>{item.name}</span>
                {item.id === selectedId && <span className="nav-dot" />}
              </button>
            ))}
            {!loading && workspaces.length === 0 && (
              <p className="side-empty">Create a workspace to begin.</p>
            )}
          </nav>
          <div className="sidebar-bottom">
            <div className="backend-indicator">
              <span
                className={`connection-dot ${streamState === "connected" ? "online" : streamState === "authentication-error" ? "failed" : ""}`}
              />
              <div>
                <strong>Local engine</strong>
                <small>
                  {streamState === "connected"
                    ? "Connected"
                    : streamState === "authentication-error"
                      ? "Authentication error"
                      : "Reconnecting…"}
                </small>
              </div>
            </div>
            <div className="sidebar-version">DEVORBIT / DESKTOP</div>
            <button type="button" className="sidebar-settings-button" aria-label="Settings" aria-controls="app-settings" aria-expanded={settingsOpen} onClick={() => setSettingsOpen((open) => !open)}>
              <Icon name="settings" size={18} /><span>Settings</span>
            </button>
          </div>
          {settingsOpen && <section id="app-settings" className="settings-panel" aria-label="Settings" onKeyDown={(event) => { if (event.key === "Escape") setSettingsOpen(false); }}>
            <h2>Settings</h2>
            <label className="terminal-font-control">Terminal font
              <select aria-label="Terminal font" value={terminalFont} onChange={(event) => { const family = event.target.value; setTerminalFont(family); if (family) localStorage.setItem("devorbit.terminalFont", family); else localStorage.removeItem("devorbit.terminalFont"); }}>
                <option value="">System default</option>
                {terminalFont && !terminalFonts.includes(terminalFont) && <option value={terminalFont}>{terminalFont}</option>}
                {terminalFonts.map((family) => <option key={family} value={family}>{family}</option>)}
              </select>
            </label>
          </section>}
        </aside>
        <main className="main">
          <header className="topbar">
            <div className="breadcrumbs">
              <button onClick={() => { setSelectedServiceId(null); setWorkspaceView("overview"); }}>
                Workspaces
              </button>
              {workspace && (
                <>
                  <Icon name="arrow" size={14} />
                  <button onClick={() => { setSelectedServiceId(null); setWorkspaceView("overview"); }}>
                    {workspace.name}
                  </button>
                </>
              )}
              {selectedService && (
                <>
                  <Icon name="arrow" size={14} />
                  <span>{selectedService.name}</span>
                </>
              )}
              {workspace && workspaceView === "terminals" && <><Icon name="arrow" size={14} /><span>Terminal</span></>}
            </div>
          </header>
          <div className="content">
            {error && (
              <div className="error-banner" role="alert">
                <span>{error}</span>
                <button
                  onClick={() => {
                    setError("");
                    loadWorkspaces()
                      .then(() => {
                        if (selectedRef.current)
                          return loadWorkspace(selectedRef.current);
                      })
                      .catch((error) => setError(errorMessage(error)));
                  }}
                >
                  Retry
                </button>
              </div>
            )}
            {streamState === "reconnecting" && (
              <div className="reconnect-banner" role="status">
                Connection lost. Reconnecting and refreshing state when the
                backend returns…
              </div>
            )}
            {loading ? (
              <div className="center-state">Loading workspaces…</div>
            ) : !workspace ? (
              <div className="welcome">
                <div className="welcome-art">
                  <span className="brand-icon large">
                    <span />
                  </span>
                </div>
                <span className="eyebrow">YOUR LOCAL CONTROL CENTER</span>
                <h1>
                  Give your projects
                  <br />
                  room to run.
                </h1>
                <p>
                  Create a workspace for a project directory, then add the
                  processes you want to manage.
                </p>
                <button
                  className="button primary"
                  onClick={() => setDialog({ kind: "workspace" })}
                >
                  <Icon name="plus" size={18} /> Create workspace
                </button>
              </div>
            ) : workspaceView === "terminals" ? (
              <section className="terminal-workspace" aria-label={`${workspace.name} terminal workspace`}>
                <nav className="terminal-sessions" aria-label="Terminal sessions">
                  {services.map((item) => {
                    const current = status?.services.find((value) => value.service_id === item.id);
                    return <button key={item.id} type="button" className={`terminal-session ${focusedTerminalId === item.id ? "active" : ""}`} aria-current={focusedTerminalId === item.id ? "page" : undefined} onClick={() => setActiveTerminalId(item.id)}>
                      <span className={`terminal-state-dot ${current?.state ?? "stopped"}`} />
                      <span>{item.name}</span>
                      <small>{current?.state ?? "stopped"}{current?.exit_code != null ? ` ${current.exit_code}` : ""}</small>
                    </button>;
                  })}
                  {shellSessions.map((session) => <button key={session.id} type="button" className={`terminal-session ${focusedTerminalId === session.id ? "active" : ""}`} aria-current={focusedTerminalId === session.id ? "page" : undefined} onClick={() => setActiveTerminalId(session.id)}><span className={`terminal-state-dot ${session.state}`} /><span>{session.shell}</span><small>{session.state}</small></button>)}
                  <button type="button" className="terminal-session-add" disabled={!!busy} onClick={createShellTerminal} aria-label="New terminal" title="New terminal"><Icon name="plus" size={15} /></button>
                  <div className="terminal-session-tools">
                    {activeTerminalService && activeTerminalStatus?.execution_id && activeTerminalStatus.execution_id !== "00000000-0000-0000-0000-000000000000" && <>
                      <button type="button" title="tmux: split left and right (Ctrl+B, %)" onClick={() => invoke("terminal_input", { serviceId: activeTerminalService.id, executionId: activeTerminalStatus.execution_id, data: "\x02%" }).catch((error) => setError(errorMessage(error)))}>Split right</button>
                      <button type="button" title={'tmux: split top and bottom (Ctrl+B, ")'} onClick={() => invoke("terminal_input", { serviceId: activeTerminalService.id, executionId: activeTerminalStatus.execution_id, data: "\x02\"" }).catch((error) => setError(errorMessage(error)))}>Split down</button>
                    </>}
                    {activeShell && <button type="button" disabled={!!busy} onClick={() => closeShellTerminal(activeShell.id)}>Close shell</button>}
                  </div>
                </nav>
                {activeTerminalService ? <TerminalPane key={`${activeTerminalService.id}:${activeTerminalStatus?.execution_id ?? ""}`} service={activeTerminalService} status={activeTerminalStatus} workspaceId={workspace.id} fontFamily={terminalFont} focused expanded busy={!!busy} onFocus={() => {}} onAction={(action) => serviceAction(activeTerminalService.id, action)} onStopped={() => loadWorkspace(workspace.id).catch((error) => setError(errorMessage(error)))} />
                  : activeShell ? <TerminalPane key={activeShell.id} shell={activeShell} workspaceId={workspace.id} fontFamily={terminalFont} focused expanded busy={!!busy} onFocus={() => {}} onStopped={() => loadShells(workspace.id).catch((error) => setError(errorMessage(error)))} />
                  : <div className="terminal-empty">Start a service or open a terminal to begin.</div>}
              </section>
            ) : selectedService ? (
              <>
                <div className="detail-back">
                  <button
                    className="text-button"
                    onClick={() => setSelectedServiceId(null)}
                  >
                    <Icon name="back" size={15} /> Workspace overview
                  </button>
                </div>
                <div className="detail-head">
                  <div className="service-emblem">
                    <Icon name="terminal" size={26} />
                  </div>
                  <div className="detail-copy">
                    <span className="eyebrow">
                      SERVICE / {workspace.name.toUpperCase()}
                    </span>
                    <h1><span>{selectedService.name}</span><GitBranch branch={selectedService.git_branch} /></h1>
                    <div className="detail-meta">
                      <StateBadge state={selectedStatus?.state ?? "stopped"} />
                      <span className="meta-divider" />
                      <span>
                        {selectedService.enabled ? "Enabled" : "Disabled"}
                      </span>
                      {selectedStatus?.pid && (
                        <>
                          <span className="meta-divider" />
                          <span>PID {selectedStatus.pid}</span>
                        </>
                      )}
                    </div>
                  </div>
                  <div className="detail-actions">
                    <button
                      className="button subtle"
                      onClick={() =>
                        setDialog({ kind: "service", item: selectedService })
                      }
                    >
                      <Icon name="edit" size={16} /> Edit
                    </button>
                    {selectedStatus?.state === "running" ||
                    selectedStatus?.state === "starting" ? (
                      <button
                        className="button danger"
                        disabled={!!busy}
                        onClick={() =>
                          serviceAction(selectedService.id, "stop")
                        }
                      >
                        <Icon name="stop" size={16} /> Stop
                      </button>
                    ) : (
                      <button
                        className="button primary"
                        disabled={!!busy || !selectedService.enabled}
                        onClick={() =>
                          serviceAction(selectedService.id, "start")
                        }
                      >
                        <Icon name="play" size={16} /> Start
                      </button>
                    )}
                    <button
                      className="button subtle"
                      disabled={!!busy || !selectedService.enabled}
                      onClick={() =>
                        serviceAction(selectedService.id, "restart")
                      }
                    >
                      <Icon name="restart" size={16} /> Restart
                    </button>
                  </div>
                </div>
                <div className="detail-tabs" role="tablist" aria-label="Service details">
                  <button type="button" role="tab" aria-selected={serviceDetailTab === "logs"} className={`tab ${serviceDetailTab === "logs" ? "active" : ""}`} onClick={() => setServiceDetailTab("logs")}><Icon name="logs" size={16} /> Logs</button>
                  <button type="button" role="tab" aria-selected={serviceDetailTab === "environment"} className={`tab ${serviceDetailTab === "environment" ? "active" : ""}`} onClick={() => setServiceDetailTab("environment")}>Environment{Object.entries(envDrafts).some(([key, value]) => key.startsWith(`${workspace.id}:${selectedService.id}:`) && value.draft !== value.content) ? " •" : ""}</button>
                  <button type="button" role="tab" aria-selected={serviceDetailTab === "traffic"} className={`tab ${serviceDetailTab === "traffic" ? "active" : ""}`} onClick={() => setServiceDetailTab("traffic")}>HTTP traffic</button>
                </div>
                {selectedStatus?.last_error && (
                  <div className="error-banner" role="status">
                    {selectedStatus.last_error}
                  </div>
                )}
                {serviceDetailTab === "logs" && <LogViewer
                  page={page}
                  entries={entries}
                  loading={logsLoading}
                  streamState={streamState}
                  onOlder={olderLogs}
                />}
                <EnvironmentEditor key={`${workspace.id}:${selectedService.id}`} workspaceId={workspace.id} serviceId={selectedService.id} active={serviceDetailTab === "environment"} drafts={envDrafts} setDrafts={setEnvDrafts} selections={envSelections} setSelections={setEnvSelections} />
                {serviceDetailTab === "traffic" && <TrafficInspector key={`${workspace.id}:${selectedService.id}`} workspaceId={workspace.id} kind="local" serviceKey={selectedService.id} ports={Object.entries(selectedService.ports ?? {}).filter(([name]) => /^(http|api|web)/i.test(name)).map(([name, number]) => ({ key: name, label: `${name} · ${number}` }))} />}
                <div className="config-strip">
                  <div>
                    <span className="eyebrow">COMMAND</span>
                    <code>
                      {formatCommandLine([selectedService.command, ...selectedService.args])}
                    </code>
                  </div>
                  <div>
                    <span className="eyebrow">WORKING DIRECTORY</span>
                    <code>
                      {selectedService.working_directory || workspace.root_path}
                    </code>
                  </div>
                  <button
                    className="text-button danger-text"
                    onClick={() => deleteService(selectedService)}
                    disabled={!!busy}
                  >
                    <Icon name="trash" size={15} /> Delete service
                  </button>
                </div>
              </>
            ) : (
              <>
                <div className="overview-head">
                  <div>
                    <span className="eyebrow">WORKSPACE OVERVIEW</span>
                    <h1>{workspace.name}</h1>
                    <p>
                      {workspace.description ||
                        "Local processes for this workspace"}
                    </p>
                  </div>
                  <div className="overview-head-actions">
                    <button className="button primary" onClick={() => setWorkspaceView("terminals")}><Icon name="terminal" size={17} /> Terminals</button>
                    <button className="button subtle" onClick={() => setDialog({ kind: "configuration" })}><Icon name="file" size={17} /> Workspace YAML</button>
                    <button className="button subtle" onClick={() => setDialog({ kind: "workspace", item: workspace })}><Icon name="edit" size={17} /> Edit workspace</button>
                  </div>
                </div>
                <div className="root-path">
                  <Icon name="folder" size={16} />
                  <span title={`${workspace.root_path}/.devorbit/workspace.yaml`}>{workspace.root_path}/.devorbit/workspace.yaml</span>
                </div>
                <div className="overview-cards">
                  <div className="summary-card">
                    <div className="summary-icon blue">
                      <Icon name="grid" size={22} />
                    </div>
                    <div>
                      <small>TOTAL SERVICES</small>
                      <strong>
                        {status?.total_services ?? services.length}
                      </strong>
                    </div>
                  </div>
                  <div className="summary-card">
                    <div className="summary-icon green">
                      <Icon name="pulse" size={22} />
                    </div>
                    <div>
                      <small>RUNNING</small>
                      <strong>{status?.running_services ?? "—"}</strong>
                    </div>
                  </div>
                  <div className="summary-card">
                    <div className="summary-icon amber">
                      <Icon name="stop" size={21} />
                    </div>
                    <div>
                      <small>WORKSPACE STATE</small>
                      <div className="card-state">
                        <StateBadge state={status?.aggregate_state ?? "idle"} />
                      </div>
                    </div>
                  </div>
                </div>
                <div className="action-row">
                  <button
                    className="button primary"
                    disabled={
                      !!busy || workspaceLoading || services.length === 0
                    }
                    onClick={() => workspaceAction("start")}
                  >
                    <Icon name="play" size={18} />
                    {busy === "workspace-start" ? "Starting…" : "Start all"}
                  </button>
                  <button
                    className="button"
                    disabled={
                      !!busy || workspaceLoading || (status !== null && status.running_services === 0 && status.starting_services === 0 && status.stopping_services === 0)
                    }
                    onClick={() => workspaceAction("stop")}
                  >
                    <Icon name="stop" size={18} />
                    {busy === "workspace-stop" ? "Stopping…" : "Stop all"}
                  </button>
                  <button
                    className="button outline"
                    disabled={
                      !!busy || workspaceLoading || services.length === 0 ||
                      (status !== null && status.running_services === 0 && status.starting_services === 0 && status.stopping_services === 0)
                    }
                    onClick={() => workspaceAction("restart")}
                  >
                    <Icon name="restart" size={18} />
                    {busy === "workspace-restart" ? "Restarting…" : "Restart all"}
                  </button>
                  <span className="action-hint">
                    Running means launched, not necessarily ready.
                  </span>
                </div>
                {result && (
                  <div className="operation-result" role="status">
                    <div className="result-heading">
                      <strong>
                        {result.operation === "start"
                          ? "Start all"
                          : result.operation === "restart"
                            ? "Restart all"
                            : "Stop all"}{" "}
                        results
                      </strong>
                      <span>
                        {result.succeeded} completed · {result.skipped} skipped
                        · {result.failed} failed
                      </span>
                      <button
                        className="icon-button"
                        onClick={() => setResult(null)}
                        aria-label="Dismiss results"
                      >
                        ×
                      </button>
                    </div>
                    <div className="result-list">
                      {result.results.map((item) => (
                        <div key={item.service_id}>
                          <span>{item.service_name}</span>
                          <span
                            className={
                              item.outcome === "failed" ||
                              item.outcome === "conflict"
                                ? "result-failed"
                                : ""
                            }
                          >
                            {item.outcome.replace("_", " ")}
                            {item.error ? ` · ${item.error}` : ""}
                          </span>
                        </div>
                      ))}
                    </div>
                  </div>
                )}
                <div className="section-header">
                  <div>
                    <h2>
                      Services{" "}
                      <span className="count-pill">{services.length}</span>
                    </h2>
                  </div>
                  <button
                    className="button"
                    onClick={() => setDialog({ kind: "service" })}
                  >
                    <Icon name="plus" size={17} /> Add service
                  </button>
                </div>
                <div className="service-panel">
                  <div className="service-panel-toolbar">
                    <div className="service-label">
                      <span className="panel-marker" /> Managed services
                    </div>
                    <label className="search-box">
                      <Icon name="search" size={16} />
                      <input
                        aria-label="Search services"
                        value={search}
                        onChange={(e) => setSearch(e.target.value)}
                        placeholder="Search services"
                      />
                    </label>
                  </div>
                  {workspaceLoading ? (
                    <div className="table-empty">Loading services…</div>
                  ) : services.length === 0 ? (
                    <div className="table-empty">
                      <div className="empty-icon">
                        <Icon name="terminal" size={24} />
                      </div>
                      <h3>No services yet</h3>
                      <p>Add a command to manage it from this workspace.</p>
                      <button
                        className="button primary"
                        onClick={() => setDialog({ kind: "service" })}
                      >
                        <Icon name="plus" size={16} /> Add first service
                      </button>
                    </div>
                  ) : filteredServices.length === 0 ? (
                    <div className="table-empty">
                      No services match “{search}”.
                    </div>
                  ) : (
                    <>
                      <div className="service-table-head">
                        <span>SERVICE</span>
                        <span>STATUS</span>
                        <span title="Listening TCP ports detected from the service process and its children">PORT</span>
                        <span>COMMAND</span>
                        <span>ACTIONS</span>
                      </div>
                      {filteredServices.map((item) => {
                        const current = status?.services.find(
                          (value) => value.service_id === item.id,
                        );
                        const commandLine = formatCommandLine([item.command, ...item.args]);
                        const active =
                          current?.state === "running" ||
                          current?.state === "starting";
                        return (
                          <div className="service-row" key={item.id}>
                            <button
                              className="service-name"
                              onClick={() => setSelectedServiceId(item.id)}
                            >
                              <span className="service-icon">
                                <Icon name="terminal" size={19} />
                              </span>
                              <span className="service-name-copy">
                                <span className="service-name-line"><strong>{item.name}</strong><GitBranch branch={item.git_branch} /></span>
                                <small>{item.enabled ? "Enabled" : "Disabled"}</small>
                              </span>
                            </button>
                            <div>
                              <StateBadge state={current?.state ?? "stopped"} />
                            </div>
                            <div className="service-ports">
                              {current?.state === "running" && current.listening_ports?.length
                                ? current.listening_ports.map((port) => (
                                    <span key={port}>:{port}</span>
                                  ))
                                : <span className="port-empty">—</span>}
                            </div>
                            <code title={commandLine}>{commandLine}</code>
                            <div className="row-actions">
                              {active ? (
                                <button
                                  className="icon-button"
                                  title="Stop"
                                  aria-label={`Stop ${item.name}`}
                                  disabled={!!busy}
                                  onClick={() => serviceAction(item.id, "stop")}
                                >
                                  <Icon name="stop" size={17} />
                                </button>
                              ) : (
                                <button
                                  className="icon-button"
                                  title="Start"
                                  aria-label={`Start ${item.name}`}
                                  disabled={!!busy || !item.enabled}
                                  onClick={() =>
                                    serviceAction(item.id, "start")
                                  }
                                >
                                  <Icon name="play" size={17} />
                                </button>
                              )}
                              <button
                                className="icon-button"
                                title="Restart"
                                aria-label={`Restart ${item.name}`}
                                disabled={!!busy || !item.enabled}
                                onClick={() =>
                                  serviceAction(item.id, "restart")
                                }
                              >
                                <Icon name="restart" size={17} />
                              </button>
                              <button
                                className="icon-button"
                                title="Open details"
                                aria-label={`Open ${item.name}`}
                                onClick={() => setSelectedServiceId(item.id)}
                              >
                                <Icon name="arrow" size={17} />
                              </button>
                            </div>
                          </div>
                        );
                      })}
                    </>
                  )}
                </div>
                <DockerPanel key={workspace.id} workspaceId={workspace.id} />
                <div className="workspace-footer">
                  <span>Service configurations run as your local user.</span>
                  <button
                    className="text-button danger-text"
                    disabled={!!busy}
                    onClick={() => deleteWorkspace(workspace)}
                  >
                    <Icon name="trash" size={15} /> Delete workspace
                  </button>
                </div>
              </>
            )}
          </div>
        </main>
      </div>
      {dialog?.kind === "workspace" && (
        <WorkspaceForm
          item={dialog.item}
          onSave={saveWorkspace}
          onClose={() => setDialog(null)}
        />
      )}
      {dialog?.kind === "service" && workspace && (
        <ServiceForm
          item={dialog.item}
          workspaceRoot={workspace.root_path}
          onSave={saveService}
          onClose={() => setDialog(null)}
        />
      )}
      {dialog?.kind === "configuration" && workspace && configuration && (
        <ConfigurationEditor workspaceId={workspace.id} initial={configuration} onSaved={async () => { await loadWorkspace(workspace.id); setDialog(null); }} onClose={() => setDialog(null)} />
      )}
    </>
  );
}
