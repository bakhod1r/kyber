import { formatDate } from "../../../shared/i18n/labels";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { agileUseCases, type Version } from "../application";
import { useT } from "../../../shared/i18n/i18n";

const day = (iso: string | null) => (iso ? formatDate(iso) : "—");

/** Releases: the project's versions (Jira's fix versions) — progress, release, archive, notes. */
export function Releases({ projectKey }: { projectKey: string }) {
  const t = useT();
  const qc = useQueryClient();
  const key = ["versions", projectKey];
  const versions = useQuery({ queryKey: key, queryFn: () => agileUseCases.versions(projectKey) });
  const [name, setName] = useState("");
  const [date, setDate] = useState("");
  const [notice, setNotice] = useState<{ kind: "ok" | "error"; text: string } | null>(null);
  const [notesOf, setNotesOf] = useState<Version | null>(null);
  const notes = useQuery({ queryKey: ["release-notes", notesOf?.id], queryFn: () => agileUseCases.releaseNotes(notesOf!.id), enabled: !!notesOf });

  const refresh = (text: string) => {
    setNotice({ kind: "ok", text });
    return qc.invalidateQueries({ queryKey: key });
  };
  const failed = (err: Error) => setNotice({ kind: "error", text: err.message });
  const create = useMutation({
    mutationFn: () => agileUseCases.createVersion(projectKey, { name, description: "", release_date: date ? `${date}T00:00:00.000Z` : null }),
    onSuccess: (v) => {
      setName("");
      setDate("");
      return refresh(t("{name} created", { name: v.name }));
    },
    onError: failed,
  });
  const release = useMutation({
    mutationFn: ({ v, moveTo }: { v: Version; moveTo: string }) => agileUseCases.releaseVersion(v.id, moveTo),
    onSuccess: (r) => refresh(r.moved ? t("{name} released; {n} unresolved moved on", { name: r.version.name, n: r.moved }) : t("{name} released", { name: r.version.name })),
    onError: failed,
  });
  const step = useMutation({
    mutationFn: ({ v, to }: { v: Version; to: "unrelease" | "archive" | "restore" }) => agileUseCases.versionStep(v.id, to),
    onSuccess: (v) => refresh(t("{name} is {state}", { name: v.name, state: v.state })),
    onError: failed,
  });
  const remove = useMutation({
    mutationFn: (v: Version) => agileUseCases.deleteVersion(v.id),
    onSuccess: () => refresh(t("Version deleted")),
    onError: failed,
  });

  function onCreate(e: FormEvent) {
    e.preventDefault();
    if (name.trim()) create.mutate();
  }

  if (versions.isPending) return <p className="muted">{t("Loading…")}</p>;
  if (versions.isError) return <p role="alert" className="error">{versions.error.message}</p>;
  const unreleased = versions.data.filter((v) => v.state === "unreleased");
  return (
    <section className="panel releases" aria-label={t("Releases")}>
      <h2>{t("Releases")}</h2>
      <form className="row" onSubmit={onCreate}>
        <label>
          {t("Version name")}
          <input value={name} onChange={(e) => setName(e.target.value)} maxLength={60} />
        </label>
        <label>
          {t("Release date")}
          <input type="date" value={date} onChange={(e) => setDate(e.target.value)} />
        </label>
        <button type="submit" disabled={create.isPending || !name.trim()}>
          {t("Create version")}
        </button>
      </form>
      {notice && (
        <p role={notice.kind === "error" ? "alert" : "status"} className={notice.kind === "error" ? "error" : "muted"}>
          {notice.text}
        </p>
      )}
      {versions.data.length === 0 ? (
        <p className="muted">{t("No versions yet. Create one and plan issues into it from the issue panel.")}</p>
      ) : (
        <table className="versions-table">
          <thead>
            <tr>
              <th scope="col">{t("Version")}</th>
              <th scope="col">{t("Status")}</th>
              <th scope="col">{t("Progress")}</th>
              <th scope="col">{t("Release date")}</th>
              <th scope="col">{t("Actions")}</th>
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
        <section aria-label={t("Release notes")} className="release-notes">
          <div className="row">
            <h3>{t("Release notes — {name}", { name: notesOf.name })}</h3>
            <button type="button" className="ghost" onClick={() => setNotesOf(null)}>
              {t("Close")}
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
  const t = useT();
  const [moveTo, setMoveTo] = useState("");
  const total = v.issues ?? 0;
  const done = v.done ?? 0;
  return (
    <tr>
      <th scope="row">{v.name}</th>
      <td>{v.state}</td>
      <td>
        <meter min={0} max={Math.max(total, 1)} value={done} aria-label={t("{name} progress", { name: v.name })} /> {t("{done}/{total} done", { done, total })}
        {(v.unresolved ?? 0) > 0 && <span className="muted"> · {t("{n} unresolved", { n: v.unresolved ?? 0 })}</span>}
      </td>
      <td>{day(v.state === "released" ? v.released_at : v.release_date)}</td>
      <td className="row">
        {v.state === "unreleased" && (
          <>
            {(v.unresolved ?? 0) > 0 && others.length > 0 && (
              <label>
                {t("Unresolved to")}
                <select value={moveTo} onChange={(e) => setMoveTo(e.target.value)}>
                  <option value="">{t("Keep here")}</option>
                  {others.map((o) => (
                    <option key={o.id} value={o.id}>
                      {o.name}
                    </option>
                  ))}
                </select>
              </label>
            )}
            <button type="button" onClick={() => onRelease(moveTo)}>
              {t("Release {name}", { name: v.name })}
            </button>
          </>
        )}
        {v.state === "released" && (
          <button type="button" className="ghost" onClick={() => onStep("unrelease")}>
            {t("Unrelease")}
          </button>
        )}
        {v.state === "archived" ? (
          <button type="button" className="ghost" onClick={() => onStep("restore")}>
            {t("Restore")}
          </button>
        ) : (
          <button type="button" className="ghost" onClick={() => onStep("archive")}>
            {t("Archive")}
          </button>
        )}
        <button type="button" className="ghost" onClick={onNotes}>
          {t("Release notes")}
        </button>
        <button type="button" className="ghost" aria-label={t("Delete {name}", { name: v.name })} onClick={onDelete}>
          {t("Delete")}
        </button>
      </td>
    </tr>
  );
}
