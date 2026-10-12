import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { type Version, api } from "../api";

const day = (iso: string | null) => (iso ? new Date(iso).toLocaleDateString() : "—");

/** Releases: the project's versions (Jira's fix versions) — progress, release, archive, notes. */
export function Releases({ projectKey }: { projectKey: string }) {
  const qc = useQueryClient();
  const key = ["versions", projectKey];
  const versions = useQuery({ queryKey: key, queryFn: () => api.versions(projectKey) });
  const [name, setName] = useState("");
  const [date, setDate] = useState("");
  const [notice, setNotice] = useState<{ kind: "ok" | "error"; text: string } | null>(null);
  const [notesOf, setNotesOf] = useState<Version | null>(null);
  const notes = useQuery({ queryKey: ["release-notes", notesOf?.id], queryFn: () => api.releaseNotes(notesOf!.id), enabled: !!notesOf });

  const refresh = (text: string) => {
    setNotice({ kind: "ok", text });
    return qc.invalidateQueries({ queryKey: key });
  };
  const failed = (err: Error) => setNotice({ kind: "error", text: err.message });
  const create = useMutation({
    mutationFn: () => api.createVersion(projectKey, { name, description: "", release_date: date ? `${date}T00:00:00.000Z` : null }),
    onSuccess: (v) => {
      setName("");
      setDate("");
      return refresh(`${v.name} created`);
    },
    onError: failed,
  });
  const release = useMutation({
    mutationFn: ({ v, moveTo }: { v: Version; moveTo: string }) => api.releaseVersion(v.id, moveTo),
    onSuccess: (r) => refresh(`${r.version.name} released${r.moved ? `; ${r.moved} unresolved moved on` : ""}`),
    onError: failed,
  });
  const step = useMutation({
    mutationFn: ({ v, to }: { v: Version; to: "unrelease" | "archive" | "restore" }) => api.versionStep(v.id, to),
    onSuccess: (v) => refresh(`${v.name} is ${v.state}`),
    onError: failed,
  });
  const remove = useMutation({
    mutationFn: (v: Version) => api.deleteVersion(v.id),
    onSuccess: () => refresh("Version deleted"),
    onError: failed,
  });

  function onCreate(e: FormEvent) {
    e.preventDefault();
    if (name.trim()) create.mutate();
  }

  if (versions.isPending) return <p className="muted">Loading…</p>;
  if (versions.isError) return <p role="alert" className="error">{versions.error.message}</p>;
  const unreleased = versions.data.filter((v) => v.state === "unreleased");
  return (
    <section className="panel releases" aria-label="Releases">
      <h2>Releases</h2>
      <form className="row" onSubmit={onCreate}>
        <label>
          Version name
          <input value={name} onChange={(e) => setName(e.target.value)} maxLength={60} />
        </label>
        <label>
          Release date
          <input type="date" value={date} onChange={(e) => setDate(e.target.value)} />
        </label>
        <button type="submit" disabled={create.isPending || !name.trim()}>
          Create version
        </button>
      </form>
      {notice && (
        <p role={notice.kind === "error" ? "alert" : "status"} className={notice.kind === "error" ? "error" : "muted"}>
          {notice.text}
        </p>
      )}
      {versions.data.length === 0 ? (
        <p className="muted">No versions yet. Create one and plan issues into it from the issue panel.</p>
      ) : (
        <table className="versions-table">
          <thead>
            <tr>
              <th scope="col">Version</th>
              <th scope="col">Status</th>
              <th scope="col">Progress</th>
              <th scope="col">Release date</th>
              <th scope="col">Actions</th>
            </tr>
          </thead>
          <tbody>
            {versions.data.map((v) => (
              <VersionRow
                key={v.id}
                v={v}
                others={unreleased.filter((o) => o.id !== v.id)}
                onRelease={(moveTo) => release.mutate({ v, moveTo })}
                onStep={(to) => step.mutate({ v, to })}
                onDelete={() => remove.mutate(v)}
                onNotes={() => setNotesOf(v)}
              />
            ))}
          </tbody>
        </table>
      )}
      {notesOf && (
        <section aria-label="Release notes" className="release-notes">
          <div className="row">
            <h3>Release notes — {notesOf.name}</h3>
            <button type="button" className="ghost" onClick={() => setNotesOf(null)}>
              Close
            </button>
          </div>
          {notes.data !== undefined && <pre>{notes.data}</pre>}
          {notes.isError && <p role="alert" className="error">{notes.error.message}</p>}
        </section>
      )}
    </section>
  );
}

function VersionRow({
  v,
  others,
  onRelease,
  onStep,
  onDelete,
  onNotes,
}: {
  v: Version;
  others: Version[];
  onRelease: (moveTo: string) => void;
  onStep: (to: "unrelease" | "archive" | "restore") => void;
  onDelete: () => void;
  onNotes: () => void;
}) {
  const [moveTo, setMoveTo] = useState("");
  const total = v.issues ?? 0;
  const done = v.done ?? 0;
  return (
    <tr>
      <th scope="row">{v.name}</th>
      <td>{v.state}</td>
      <td>
        <meter min={0} max={Math.max(total, 1)} value={done} aria-label={`${v.name} progress`} /> {done}/{total} done
        {(v.unresolved ?? 0) > 0 && <span className="muted"> · {v.unresolved} unresolved</span>}
      </td>
      <td>{day(v.state === "released" ? v.released_at : v.release_date)}</td>
      <td className="row">
        {v.state === "unreleased" && (
          <>
            {(v.unresolved ?? 0) > 0 && others.length > 0 && (
              <label>
                Unresolved to
                <select value={moveTo} onChange={(e) => setMoveTo(e.target.value)}>
                  <option value="">Keep here</option>
                  {others.map((o) => (
                    <option key={o.id} value={o.id}>
                      {o.name}
                    </option>
                  ))}
                </select>
              </label>
            )}
            <button type="button" onClick={() => onRelease(moveTo)}>
              Release {v.name}
            </button>
          </>
        )}
        {v.state === "released" && (
          <button type="button" className="ghost" onClick={() => onStep("unrelease")}>
            Unrelease
          </button>
        )}
        {v.state === "archived" ? (
          <button type="button" className="ghost" onClick={() => onStep("restore")}>
            Restore
          </button>
        ) : (
          <button type="button" className="ghost" onClick={() => onStep("archive")}>
            Archive
          </button>
        )}
        <button type="button" className="ghost" onClick={onNotes}>
          Release notes
        </button>
        <button type="button" className="ghost" aria-label={`Delete ${v.name}`} onClick={onDelete}>
          Delete
        </button>
      </td>
    </tr>
  );
}
