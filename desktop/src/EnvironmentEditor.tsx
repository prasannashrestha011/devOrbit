import { useCallback, useEffect, useState, type Dispatch, type SetStateAction } from "react";
import { api } from "./api";
import EnvSource from "./EnvSource";

export type EnvDraft = { content: string; draft: string; revision: string };

export default function EnvironmentEditor({ workspaceId, serviceId, active, drafts, setDrafts, selections, setSelections }: {
  workspaceId: string;
  serviceId: string;
  active: boolean;
  drafts: Record<string, EnvDraft>;
  setDrafts: Dispatch<SetStateAction<Record<string, EnvDraft>>>;
  selections: Record<string, string>;
  setSelections: Dispatch<SetStateAction<Record<string, string>>>;
}) {
  const scope = `${workspaceId}:${serviceId}`;
  const [files, setFiles] = useState<string[] | null>(null);
  const [loadingList, setLoadingList] = useState(true);
  const [loadingFile, setLoadingFile] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const selected = files?.includes(selections[scope]) ? selections[scope] : files?.[0];
  const key = `${scope}:${selected}`;
  const current = drafts[key];
  const dirty = current && current.draft !== current.content;

  const refresh = useCallback(async () => {
    setLoadingList(true);
    setError("");
    try {
      setFiles(await api.environmentFiles(serviceId));
    } catch (problem) {
      setError(problem instanceof Error ? problem.message : String(problem));
    } finally {
      setLoadingList(false);
    }
  }, [serviceId]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useEffect(() => {
    if (!active || !selected) return;
    let mounted = true;
    setLoadingFile(true);
    setError("");
    setNotice("");
    api.environmentFile(serviceId, selected)
      .then((file) => {
        if (!mounted) return;
        setDrafts((all) => {
          const previous = all[key];
          if (previous && previous.draft !== previous.content) return all;
          return { ...all, [key]: {
            content: file.content,
            revision: file.revision,
            draft: file.content,
          } };
        });
      })
      .catch((problem) => {
        if (mounted) setError(problem instanceof Error ? problem.message : String(problem));
      })
      .finally(() => {
        if (mounted) setLoadingFile(false);
      });
    return () => { mounted = false; };
  }, [active, serviceId, selected, key, setDrafts]);

  async function reload() {
    if (!selected) return;
    if (dirty && !window.confirm(`Discard unsaved changes to ${selected}?`)) return;
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const file = await api.environmentFile(serviceId, selected);
      setDrafts((all) => ({ ...all, [key]: { content: file.content, draft: file.content, revision: file.revision } }));
    } catch (problem) {
      setError(problem instanceof Error ? problem.message : String(problem));
    } finally {
      setBusy(false);
    }
  }

  async function save() {
    if (!selected || !current || !dirty || busy) return;
    setBusy(true);
    setError("");
    setNotice("");
    const submitted = current.draft;
    try {
      const file = await api.saveEnvironmentFile(serviceId, selected, submitted, current.revision);
      setDrafts((all) => ({ ...all, [key]: {
        content: file.content,
        revision: file.revision,
        draft: all[key]?.draft === submitted ? file.content : all[key].draft,
      } }));
      setNotice("Environment file saved. Restart the service separately to use changes.");
    } catch (problem) {
      setError(problem instanceof Error ? problem.message : String(problem));
    } finally {
      setBusy(false);
    }
  }

  return <section className="env-panel" aria-label="Environment files" hidden={!active}>
    <div className="env-toolbar">
      <div><strong>Environment files</strong><small>From this service’s working directory</small></div>
      <button type="button" className="button subtle" disabled={loadingList || busy} onClick={() => void refresh()}>Refresh</button>
    </div>
    {loadingList && !files ? <p className="env-empty">Discovering environment files…</p> : files?.length === 0 ? <p className="env-empty">No .env files in this service’s working directory.</p> : files ? <>
      <div className="env-files" aria-label="Environment file list">{files.map((name) => {
        const item = drafts[`${scope}:${name}`];
        return <button type="button" key={name} className={selected === name ? "active" : ""} aria-pressed={selected === name} disabled={busy} onClick={() => setSelections((all) => ({ ...all, [scope]: name }))}>{name}{item && item.draft !== item.content ? " • Unsaved" : ""}</button>;
      })}</div>
      {selected && <div className="env-editor">
        <div className="env-editor-head"><div><strong>{selected}</strong><small>./{selected}</small></div>{dirty && <span className="env-unsaved">Unsaved changes</span>}</div>
        {loadingFile && !current ? <p className="env-empty">Loading file…</p> : current ? <EnvSource value={current.draft} onChange={(value) => setDrafts((all) => ({ ...all, [key]: { ...all[key], draft: value } }))} onSave={() => void save()} /> : null}
        <div className="env-actions"><button type="button" className="button subtle" disabled={busy || loadingFile} onClick={() => void reload()}>Reload</button><button type="button" className="button primary" disabled={busy || !dirty} onClick={() => void save()}>{busy ? "Saving…" : "Save"}</button></div>
      </div>}
    </> : null}
    {error && <p className="form-error" role="alert">{error}</p>}
    {notice && <p className="hint" role="status">{notice}</p>}
  </section>;
}
