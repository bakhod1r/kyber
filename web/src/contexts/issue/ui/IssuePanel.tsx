import type { ReactNode } from "react";
import { IssueChildren } from "./IssueChildren";
import { IssueHistory } from "./IssueHistory";
import { IssueLinks } from "./IssueLinks";
import { IssueWatchers } from "./IssueWatchers";
import { MentionTextarea } from "./MentionTextarea";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useEffect, useId, useState } from "react";
import { issueUseCases, type Issue, PRIORITIES, type Priority, RESOLUTIONS, type Resolution } from "../application";
import { ApiError } from "../../../shared/http";
import { agileUseCases } from "../../agile/application";
import { projectUseCases } from "../../project/application";

type Draft = { title: string; description: string; priority: Priority; assignee: string; estimate: string; confidential: boolean; resolution: Resolution | ""; fixVersions: string[] };

const toDraft = (i: Issue): Draft => ({
  title: i.title,
  description: i.description,
  priority: i.priority,
  assignee: i.assignee_id ?? "",
  estimate: i.estimate === null ? "" : String(i.estimate),
  confidential: i.security_level === "confidential",
  resolution: i.resolution ?? "",
  fixVersions: i.fix_versions ?? [],
});

export function IssuePanel({
  issueKey,
  projectKey,
  onClose,
  onOpen,
  extensions,
}: {
  issueKey: string;
  projectKey: string;
  onClose: () => void;
  onOpen?: (issueKey: string) => void;
  /** Panels other contexts contribute (e.g. Focus), composed by the app shell. */
  extensions?: ReactNode;
}) {
  const qc = useQueryClient();
  const titleId = useId();
  const issue = useQuery({ queryKey: ["issue", issueKey], queryFn: () => issueUseCases.issue(issueKey) });
  const members = useQuery({ queryKey: ["members", projectKey], queryFn: () => projectUseCases.members(projectKey) });
  const versions = useQuery({ queryKey: ["versions", projectKey], queryFn: () => agileUseCases.versions(projectKey) });
  const [draft, setDraft] = useState<Draft | null>(null);
  const [notice, setNotice] = useState<{ kind: "ok" | "error"; text: string } | null>(null);

  // Reset the form whenever a new version of the issue arrives.
  const loaded = issue.data;
  useEffect(() => {
    if (loaded) setDraft(toDraft(loaded));
  }, [loaded]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [onClose]);

  const save = useMutation({
    mutationFn: (d: Draft) =>
      issueUseCases.editIssue(issueKey, {
        version: loaded?.version ?? 0,
        title: d.title,
        description: d.description,
        priority: d.priority,
        assignee_id: d.assignee || null,
        estimate: d.estimate.trim() === "" ? null : Number(d.estimate),
        security_level: d.confidential ? "confidential" : "",
        ...(d.resolution ? { resolution: d.resolution } : {}),
        fix_versions: d.fixVersions,
      }),
    onSuccess: (updated) => {
      qc.setQueryData(["issue", issueKey], updated);
      setNotice({ kind: "ok", text: "Saved" });
      return qc.invalidateQueries({ queryKey: ["issues", projectKey] });
    },
    onError: (err) => {
      if (err instanceof ApiError && err.code === "ISSUE_CONFLICT") {
        setNotice({ kind: "error", text: "This issue was changed by someone else — the latest version is loaded." });
        void issue.refetch();
        return;
      }
      setNotice({ kind: "error", text: err.message });
    },
  });

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (draft) save.mutate(draft);
  }

  return (
    <div className="overlay" onClick={onClose}>
      <div role="dialog" aria-modal="true" aria-labelledby={titleId} className="panel issue-panel" onClick={(e) => e.stopPropagation()}>
        <header className="issue-panel-head">
          <h2 id={titleId}>
            <span className="key">{issueKey}</span> {loaded?.status.replace("_", " ")}
          </h2>
          <button type="button" className="ghost" aria-label="Close" onClick={onClose}>
            ✕
          </button>
        </header>
        {issue.isError && <p role="alert" className="error">{issue.error.message}</p>}
        {loaded && (
          <p className="muted people">
            Reporter: {loaded.reporter_id ? (members.data?.find((m) => m.user_id === loaded.reporter_id)?.name ?? "Former member") : "Unknown"}
            {loaded.parent_key && (
              <>
                {" · "}
                <span className="parent-link">
                  {loaded.type === "subtask" ? "Sub-task of " : "Epic "}
                  <button type="button" className="link" onClick={() => onOpen?.(loaded.parent_key!)}>
                    {loaded.parent_key}
                  </button>
                </span>
              </>
            )}
          </p>
        )}
        {draft && (
          <form onSubmit={onSubmit} className="issue-form">
            <label>
              Title
              <input value={draft.title} onChange={(e) => setDraft({ ...draft, title: e.target.value })} required />
            </label>
            <label>
              Description
              <textarea rows={6} value={draft.description} onChange={(e) => setDraft({ ...draft, description: e.target.value })} />
            </label>
            <div className="row">
              <label>
                Priority
                <select value={draft.priority} onChange={(e) => setDraft({ ...draft, priority: e.target.value as Priority })}>
                  {PRIORITIES.map((p) => (
                    <option key={p} value={p}>
                      {p}
                    </option>
                  ))}
                </select>
              </label>
              <label>
                Assignee
                <select value={draft.assignee} onChange={(e) => setDraft({ ...draft, assignee: e.target.value })}>
                  <option value="">Unassigned</option>
                  {members.data?.filter((m) => m.role !== "viewer").map((m) => (
                    <option key={m.user_id} value={m.user_id}>
                      {m.name}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            {draft.resolution && (
              <label className="narrow">
                Resolution
                <select value={draft.resolution} onChange={(e) => setDraft({ ...draft, resolution: e.target.value as Resolution })}>
                  {RESOLUTIONS.map((r) => (
                    <option key={r.id} value={r.id}>
                      {r.label}
                    </option>
                  ))}
                </select>
              </label>
            )}
            {(versions.data ?? []).some((v) => v.state !== "archived" || draft.fixVersions.includes(v.id)) && (
              <fieldset className="fix-versions">
                <legend>Fix versions</legend>
                {(versions.data ?? [])
                  .filter((v) => v.state !== "archived" || draft.fixVersions.includes(v.id))
                  .map((v) => (
                    <label key={v.id} className="check">
                      <input
                        type="checkbox"
                        checked={draft.fixVersions.includes(v.id)}
                        onChange={(e) =>
                          setDraft({
                            ...draft,
                            fixVersions: e.target.checked ? [...draft.fixVersions, v.id] : draft.fixVersions.filter((x) => x !== v.id),
                          })
                        }
                      />
                      {v.name}
                    </label>
                  ))}
              </fieldset>
            )}
            <label className="narrow">
              Story points
              <input type="number" min={0} max={999} step={0.5} value={draft.estimate} onChange={(e) => setDraft({ ...draft, estimate: e.target.value })} />
            </label>
            <label className="check">
              <input type="checkbox" checked={draft.confidential} onChange={(e) => setDraft({ ...draft, confidential: e.target.checked })} />
              Confidential — only admins and the people on this issue see it
            </label>
            <div className="row end">
              {notice && (
                <span role={notice.kind === "error" ? "alert" : "status"} className={notice.kind === "error" ? "error" : "muted"}>
                  {notice.text}
                </span>
              )}
              <button type="submit" disabled={save.isPending}>
                Save
              </button>
            </div>
          </form>
        )}
        {loaded && <IssueChildren issue={loaded} projectKey={projectKey} onOpen={onOpen} />}
        <IssueWatchers issueKey={issueKey} />
        <IssueLinks issueKey={issueKey} onOpen={onOpen} />
        {extensions}
        <Comments issueKey={issueKey} projectKey={projectKey} />
        <IssueHistory issueKey={issueKey} projectKey={projectKey} />
      </div>
    </div>
  );
}

function Comments({ issueKey, projectKey }: { issueKey: string; projectKey: string }) {
  const members = useQuery({ queryKey: ["members", projectKey], queryFn: () => projectUseCases.members(projectKey) });
  const qc = useQueryClient();
  const comments = useQuery({ queryKey: ["comments", issueKey], queryFn: () => issueUseCases.comments(issueKey) });
  const [body, setBody] = useState("");
  const add = useMutation({
    mutationFn: () => issueUseCases.addComment(issueKey, body),
    onSuccess: () => {
      setBody("");
      return qc.invalidateQueries({ queryKey: ["comments", issueKey] });
    },
  });

  return (
    <section className="comments" aria-label="Comments">
      <h3>Comments</h3>
      <ul>
        {comments.data?.map((c) => (
          <li key={c.id}>
            <div className="comment-meta">
              <strong>{c.author_name}</strong> <time dateTime={c.created_at}>{new Date(c.created_at).toLocaleString()}</time>
            </div>
            <p className="comment-body">{c.body}</p>
          </li>
        ))}
      </ul>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (body.trim()) add.mutate();
        }}
      >
        <MentionTextarea
          label="Add a comment"
          value={body}
          onChange={setBody}
          members={members.data ?? []}
          placeholder="Write a comment. Type @ to mention someone"
        />
        {add.isError && <p role="alert" className="error">{add.error.message}</p>}
        <button type="submit" disabled={add.isPending}>
          Comment
        </button>
      </form>
    </section>
  );
}
