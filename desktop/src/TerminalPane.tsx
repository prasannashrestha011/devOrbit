import { useEffect, useRef, useState } from "react";
import { Channel, invoke } from "@tauri-apps/api/core";
import type { Terminal } from "@xterm/xterm";
import type { FitAddon } from "@xterm/addon-fit";
import type { SearchAddon } from "@xterm/addon-search";
import type { DockerTerminal, ProcessStatus, Service, ShellSession } from "./api";

type Frame = { type: string; sequence?: number; data?: string };

export default function TerminalPane({ service, status, shell, docker, workspaceId, fontFamily = "", focused, expanded, busy, onFocus, onExpand, onAction, onClose, onStopped }: {
  service?: Service;
  status?: ProcessStatus;
  shell?: ShellSession;
  docker?: DockerTerminal;
  workspaceId?: string;
  fontFamily?: string;
  focused: boolean;
  expanded: boolean;
  busy: boolean;
  onFocus: () => void;
  onExpand?: () => void;
  onAction?: (action: "start" | "stop") => void;
  onClose?: () => void;
  onStopped?: () => void;
}) {
  const host = useRef<HTMLDivElement>(null);
  const terminal = useRef<Terminal | null>(null);
  const searchAddon = useRef<SearchAddon | null>(null);
  const fitAddon = useRef<FitAddon | null>(null);
  const resizeRef = useRef<() => void>(() => {});
  const fontFamilyRef = useRef(fontFamily);
  fontFamilyRef.current = fontFamily;
  const acceptsInput = useRef(false);
  const isRunning = useRef(false);
  const inputQueue = useRef<Promise<void>>(Promise.resolve());
  const onStoppedRef = useRef(onStopped);
  onStoppedRef.current = onStopped;
  const focusedRef = useRef(focused);
  focusedRef.current = focused;
  const [connection, setConnection] = useState("disconnected");
  const [search, setSearch] = useState("");
  const [showSearch, setShowSearch] = useState(false);
  const id = docker?.id ?? shell?.id ?? service?.id ?? "";
  const name = docker?.container_name ?? shell?.shell ?? service?.name ?? "Terminal";
  const executionId = docker?.id ?? shell?.id ?? status?.execution_id;
  const running = (docker?.state ?? shell?.state ?? status?.state) === "running";
  isRunning.current = docker ? docker.state === "running" && connection !== "stopped" : shell ? shell.state === "running" : Boolean(service && executionId && connection !== "stopped");
  acceptsInput.current = focused && isRunning.current && connection === "connected";

  useEffect(() => {
    let disposed = false;
    const viewerId = crypto.randomUUID();
    let observer: ResizeObserver | undefined;
    let instance: Terminal | undefined;
    let resizeTimer = 0;
    const inputTarget = docker ? "docker" : shell ? "shell" : "service";
    const sendInput = (value: string) => {
      inputQueue.current = inputQueue.current.then(() => disposed ? undefined : inputTarget === "docker"
        ? invoke<void>("docker_terminal_input", { sessionId: id, data: value })
        : inputTarget === "shell" ? invoke<void>("shell_terminal_input", { sessionId: id, data: value })
        : invoke<void>("terminal_input", { serviceId: id, executionId, data: value })).catch(() => {
          if (!disposed) setConnection("disconnected");
        });
    };
    async function mount() {
      const [{ Terminal }, { FitAddon }, { SearchAddon }, { CanvasAddon }] = await Promise.all([
        import("@xterm/xterm"), import("@xterm/addon-fit"), import("@xterm/addon-search"), import("@xterm/addon-canvas"),
      ]);
      if (disposed || !host.current) return;
      instance = new Terminal({ scrollback: 1000, convertEol: false, screenReaderMode: true, cursorBlink: true, fontSize: 14, fontFamily: fontFamilyRef.current ? `${JSON.stringify(fontFamilyRef.current)}, monospace` : "monospace", theme: { background: "#080b10", foreground: "#dbe5f2", cursor: "#58d6c0" } });
      const fit = new FitAddon();
      const finder = new SearchAddon();
      instance.loadAddon(fit);
      instance.loadAddon(finder);
      // Load before open so strict release CSP never sees the DOM renderer's inline CSS.
      instance.loadAddon(new CanvasAddon());
      instance.open(host.current);
      if (focusedRef.current) instance.focus();
      terminal.current = instance;
      fitAddon.current = fit;
      searchAddon.current = finder;
      instance.write(executionId && executionId !== "00000000-0000-0000-0000-000000000000" ? "\x1b[2m[Execution started]\x1b[0m\r\n" : "\x1b[2m[Start this service to open its terminal]\x1b[0m\r\n");
      instance.onData((data) => {
        if (acceptsInput.current && executionId) {
          const encoder = new TextEncoder();
          let chunk = "";
          let bytes = 0;
          for (const character of data) {
            const size = encoder.encode(character).length;
            if (bytes + size > 4096) { sendInput(chunk); chunk = ""; bytes = 0; }
            chunk += character;
            bytes += size;
          }
          if (chunk) sendInput(chunk);
        }
      });
      const resize = () => {
        if (!host.current || !instance) return;
        fit.fit();
        if (isRunning.current && executionId && instance.cols > 0 && instance.rows > 0) {
          invoke(inputTarget === "docker" ? "docker_terminal_resize" : inputTarget === "shell" ? "shell_terminal_resize" : "terminal_resize", inputTarget === "service" ? { serviceId: id, executionId, cols: instance.cols, rows: instance.rows } : { sessionId: id, cols: instance.cols, rows: instance.rows }).catch(() => {});
        }
      };
      resizeRef.current = resize;
      observer = new ResizeObserver(() => {
        cancelAnimationFrame(resizeTimer);
        resizeTimer = requestAnimationFrame(resize);
      });
      observer.observe(host.current);
      resize();
      if (!executionId || executionId === "00000000-0000-0000-0000-000000000000") return;
      const channel = new Channel<Frame>();
      channel.onmessage = (frame) => {
        if (disposed || !instance) return;
        if (frame.type === "output" && frame.data) {
          const bytes = atob(frame.data);
          instance.write(Uint8Array.from(bytes, (char) => char.charCodeAt(0)));
        } else if (frame.type === "gap") {
          instance.write("\r\n\x1b[33m[Earlier terminal output is unavailable]\x1b[0m\r\n");
        } else if (frame.type === "stopped") {
          setConnection("stopped");
          onStoppedRef.current?.();
        } else if (frame.type === "connected") {
          setConnection(frame.type);
          resize();
        } else if (frame.type === "reconnecting" || frame.type === "authentication-error") {
          setConnection(frame.type);
        } else if (frame.type === "error") {
          setConnection("disconnected");
        }
      };
      await invoke(docker ? "start_docker_terminal" : shell ? "start_shell_terminal" : "start_terminal", docker || shell ? { workspaceId, sessionId: id, viewerId, after: 0, channel } : { serviceId: id, executionId, viewerId, after: 0, channel }).catch(() => { if (!disposed) setConnection("disconnected"); });
      if (disposed) invoke(docker ? "stop_docker_terminal" : shell ? "stop_shell_terminal" : "stop_terminal", docker || shell ? { sessionId: id, viewerId } : { serviceId: id, executionId, viewerId }).catch(() => {});
    }
    mount().catch(() => setConnection("disconnected"));
    return () => {
      disposed = true;
      observer?.disconnect();
      cancelAnimationFrame(resizeTimer);
      instance?.dispose();
      terminal.current = null;
      fitAddon.current = null;
      resizeRef.current = () => {};
      searchAddon.current = null;
      if (executionId) invoke(docker ? "stop_docker_terminal" : shell ? "stop_shell_terminal" : "stop_terminal", docker || shell ? { sessionId: id, viewerId } : { serviceId: id, executionId, viewerId }).catch(() => {});
    };
  }, [id, executionId, workspaceId]);

  useEffect(() => {
    if (focused) terminal.current?.focus();
  }, [focused]);

  useEffect(() => {
    if (!terminal.current) return;
    terminal.current.options.fontFamily = fontFamily ? `${JSON.stringify(fontFamily)}, monospace` : "monospace";
    resizeRef.current();
  }, [fontFamily]);

  function find() {
    if (search) searchAddon.current?.findNext(search);
  }

  const state = docker?.state ?? shell?.state ?? status?.state ?? "stopped";
  const shellReady = Boolean(service && (state === "exited" || state === "failed" || state === "stopped") && connection === "connected");
  return (
    <section className={`terminal-pane ${focused ? "focused" : ""} ${expanded ? "expanded" : ""}`} tabIndex={0} onFocus={onFocus} onClick={onFocus} aria-label={`${name} terminal`}>
      <header className="terminal-pane-head">
        <div className="terminal-pane-title"><span className={`terminal-state-dot ${state}`} /><strong>{name}</strong><small>{shellReady ? "tmux shell" : state}{!shellReady && (shell?.exit_code ?? status?.exit_code) != null ? ` · ${service ? "service exit" : "exit"} ${shell?.exit_code ?? status?.exit_code}` : ""}</small></div>
        <div className="terminal-pane-actions">
          <button type="button" onClick={() => setShowSearch((current) => !current)} aria-label={`Search ${name} terminal`}>Search</button>
          {shell || docker ? <button type="button" disabled={busy} onClick={onClose} aria-label={`Close ${name} terminal`}>Close</button> : <>
            {onExpand && <button type="button" onClick={onExpand} aria-label={expanded ? `Collapse ${name} terminal` : `Expand ${name} terminal`}>{expanded ? "Collapse" : "Expand"}</button>}
            <button type="button" disabled={busy || (!running && !service?.enabled)} onClick={() => onAction?.(running ? "stop" : "start")}>{running ? "Stop" : "Start"}</button>
          </>}
        </div>
      </header>
      {showSearch && <form className="terminal-search" onSubmit={(event) => { event.preventDefault(); find(); }}><input autoFocus aria-label={`Find in ${name} terminal`} value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Find in terminal" /><button type="submit">Next</button></form>}
      <div className="terminal-screen" ref={host} onClick={() => terminal.current?.focus()} />
      <footer className="terminal-pane-foot"><span>{focused && isRunning.current && connection === "connected" ? `INPUT → ${service && state !== "running" ? "tmux shell" : name}` : "Click or Tab to focus"}</span><span>{!executionId || executionId === "00000000-0000-0000-0000-000000000000" ? "Start to connect" : connection === "reconnecting" ? "Reconnecting…" : connection === "connected" ? "Connected" : connection === "stopped" ? "Read only" : connection === "authentication-error" ? "Authentication error" : "Disconnected"}</span></footer>
    </section>
  );
}
