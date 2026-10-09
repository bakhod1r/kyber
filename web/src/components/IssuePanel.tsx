import { IssueFocus } from "./Focus";
import { MentionTextarea } from "./MentionTextarea";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useEffect, useId, useState } from "react";
import { ApiError, type Issue, PRIORITIES, type Priority, api } from "../api";

type Draft = { title: string; description: string; priority: Priority; assignee: string; estimate: string };

const toDraft = (i: Issue): Draft => ({
  title: i.title,
  description: i.description,
  priority: i.priority,
  assignee: i.assignee_id ?? "",
  estimate: i.estimate === null ? "" : String(i.estimate),
});

export function IssuePanel({ issueKey, projectKey, onClose }: { issueKey: string; projectKey: string; onClose: () => void }) {
  const qc = useQueryClient();
  const titleId = useId();
  const issue = useQuery({ queryKey: ["issue", issueKey], queryFn: () => api.issue(issueKey) });
  const members = useQuery({ queryKey: ["members", projectKey], queryFn: () => api.members(projectKey) });
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
      api.editIssue(issueKey, {
        version: loaded?.version ?? 0,
        title: d.title,
        description: d.description,
        priority: d.priority,
        assignee_id: d.assignee || null,
        estimate: d.estimate.trim() === "" ? null : Number(d.estimate),
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
                  {members.data?.map((m) => (
                    <option key={m.user_id} value={m.user_id}>
                      {m.name}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            <label className="narrow">
              Story points
              <input type="number" min={0} max={999} step={0.5} value={draft.estimate} onChange={(e) => setDraft({ ...draft, estimate: e.target.value })} />
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
        <IssueFocus issueKey={issueKey} />
        <Comments issueKey={issueKey} projectKey={projectKey} />
      </div>
    </div>
  );
}

function Comments({ issueKey, projectKey }: { issueKey: string; projectKey: string }) {
  const members = useQuery({ queryKey: ["members", projectKey], queryFn: () => api.members(projectKey) });
  const qc = useQueryClient();
  const comments = useQuery({ queryKey: ["comments", issueKey], queryFn: () => api.comments(issueKey) });
  const [body, setBody] = useState("");
  const add = useMutation({
    mutationFn: () => api.addComment(issueKey, body),
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
