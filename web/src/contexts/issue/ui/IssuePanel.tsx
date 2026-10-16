import type { ReactNode } from "react";
import { IssueChildren } from "./IssueChildren";
import { IssueHistory } from "./IssueHistory";
import { IssueLinks } from "./IssueLinks";
import { IssueDetails } from "./IssueDetails";
import { IssueTimeTracking } from "./IssueTimeTracking";
import { IssueAttachments } from "./IssueAttachments";
import { IssueWatchers } from "./IssueWatchers";
import { IssuePresence } from "./IssuePresence";
import { CommentReactions } from "./CommentReactions";
import { MentionTextarea } from "./MentionTextarea";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, type KeyboardEvent as ReactKeyboardEvent, useEffect, useId, useRef, useState } from "react";
import { issueUseCases, type Issue, parseLabels, PRIORITIES, type Priority, RESOLUTIONS, type Resolution } from "../application";
import { ApiError } from "../../../shared/http";
import { agileUseCases } from "../../agile/application";
import { projectUseCases } from "../../project/application";
import { useT } from "../../../shared/i18n/i18n";

type Draft = { title: string; description: string; priority: Priority; assignee: string; estimate: string; confidential: boolean; resolution: Resolution | ""; fixVersions: string[]; labels: string };

const toDraft = (i: Issue): Draft => ({
  title: i.title,
  description: i.description,
  priority: i.priority,
  assignee: i.assignee_id ?? "",
  estimate: i.estimate === null ? "" : String(i.estimate),
  confidential: i.security_level === "confidential",
  resolution: i.resolution ?? "",
  fixVersions: i.fix_versions ?? [],
  labels: (i.labels ?? []).join(" "),
});

const FOCUSABLE = 'a[href], button, input, select, textarea, summary, [tabindex]:not([tabindex="-1"])';

export function IssuePanel({
  issueKey,
  projectKey,
  me,
  onClose,
  onOpen,
  extensions,
}: {
  issueKey: string;
  projectKey: string;
  /** The signed-in user's id, so "Also viewing" lists only the others. */
  me?: string;
  onClose: () => void;
  onOpen?: (issueKey: string) => void;
  /** Panels other contexts contribute (e.g. Focus), composed by the app shell. */
  extensions?: ReactNode;
}) {
  const t = useT();
  const qc = useQueryClient();
  const titleId = useId();
  const issue = useQuery({ queryKey: ["issue", issueKey], queryFn: () => issueUseCases.issue(issueKey) });
  const members = useQuery({ queryKey: ["members", projectKey], queryFn: () => projectUseCases.members(projectKey) });
  const versions = useQuery({ queryKey: ["versions", projectKey], queryFn: () => agileUseCases.versions(projectKey) });
  const projectLabels = useQuery({ queryKey: ["labels", projectKey], queryFn: () => issueUseCases.projectLabels(projectKey) });
  const labelsId = useId();
  const formId = useId();
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

  // A modal dialog: focus moves in on open, Tab cycles inside it, and focus returns to the opener.
  const dialogRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const opener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    dialogRef.current?.focus();
    return () => opener?.focus();
  }, []);
  function trapTab(e: ReactKeyboardEvent<HTMLDivElement>) {
    if (e.key !== "Tab" || !dialogRef.current) return;
    const focusable = [...dialogRef.current.querySelectorAll<HTMLElement>(FOCUSABLE)].filter((el) => !el.hasAttribute("disabled"));
    const first = focusable.at(0);
    const last = focusable.at(-1);
    if (!first || !last) return;
    const active = document.activeElement;
    if (e.shiftKey && (active === first || active === dialogRef.current)) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && active === last) {
      e.preventDefault();
      first.focus();
    }
  }

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
        labels: parseLabels(d.labels),
      }),
    onSuccess: (updated) => {
      qc.setQueryData(["issue", issueKey], updated);
      setNotice({ kind: "ok", text: t("Saved") });
      void qc.invalidateQueries({ queryKey: ["labels", projectKey] });
      return qc.invalidateQueries({ queryKey: ["issues", projectKey] });
    },
    onError: (err) => {
      if (err instanceof ApiError && err.code === "ISSUE_CONFLICT") {
        setNotice({ kind: "error", text: t("This issue was changed by someone else — the latest version is loaded.") });
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
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        className="panel issue-panel"
        onClick={(e) => e.stopPropagation()}
        onKeyDown={trapTab}
      >
        <header className="issue-panel-head">
          <h2 id={titleId}>
            <span className="key">{issueKey}</span> {loaded && <span className="status-pill">{loaded.status.replace("_", " ")}</span>}
          </h2>
          <button type="button" className="ghost" aria-label={t("Close")} onClick={onClose}>
            ✕
          </button>
        </header>
        <IssuePresence issueKey={issueKey} me={me} />
        {issue.isError && <p role="alert" className="error">{issue.error.message}</p>}
        {loaded && (
          <p className="muted people">
            {t("Reporter:")} {loaded.reporter_id ? (members.data?.find((m) => m.user_id === loaded.reporter_id)?.name ?? t("Former member")) : t("Unknown")}
            {loaded.parent_key && (
              <>
                {" · "}
                <span className="parent-link">
                  {loaded.type === "subtask" ? t("Sub-task of ") : t("Epic ")}
                  <button type="button" className="link" onClick={() => onOpen?.(loaded.parent_key!)}>
                    {loaded.parent_key}
                  </button>
                </span>
              </>
            )}
          </p>
        )}
        <div className="issue-layout">
          <div className="issue-main">
            {draft && (
              <form id={formId} onSubmit={onSubmit} className="issue-form">
                <label>
                  {t("Title")}
                  <input value={draft.title} onChange={(e) => setDraft({ ...draft, title: e.target.value })} required />
                </label>
                <label>
                  {t("Description")}
                  <textarea rows={6} value={draft.description} onChange={(e) => setDraft({ ...draft, description: e.target.value })} />
                </label>
              </form>
            )}
            {loaded && <IssueChildren issue={loaded} projectKey={projectKey} onOpen={onOpen} />}
            {loaded && <IssueDetails key={loaded.id} issue={loaded} projectKey={projectKey} />}
            <IssueAttachments issueKey={issueKey} />
            <IssueLinks issueKey={issueKey} onOpen={onOpen} />
            <Comments issueKey={issueKey} projectKey={projectKey} />
            <IssueHistory issueKey={issueKey} projectKey={projectKey} />
          </div>
          {/* The fields beside the text, as in Jira: they belong to the form through form={formId}. */}
          <aside className="issue-side" aria-label={t("Details")}>
            {draft && (
              <div className="issue-form">
                <div className="row">
                  <label>
                    {t("Priority")}
                    <select value={draft.priority} onChange={(e) => setDraft({ ...draft, priority: e.target.value as Priority })}>
                      {PRIORITIES.map((p) => (
                        <option key={p} value={p}>
                          {p}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label>
                    {t("Assignee")}
                    <select value={draft.assignee} onChange={(e) => setDraft({ ...draft, assignee: e.target.value })}>
                      <option value="">{t("Unassigned")}</option>
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
                    {t("Resolution")}
                    <select value={draft.resolution} onChange={(e) => setDraft({ ...draft, resolution: e.target.value as Resolution })}>
                      {RESOLUTIONS.map((r) => (
                        <option key={r.id} value={r.id}>
                          {r.label}
                        </option>
                      ))}
                    </select>
                  </label>
                )}
                <label>
                  {t("Labels")}
                  <input list={labelsId} value={draft.labels} placeholder="ui backend" onChange={(e) => setDraft({ ...draft, labels: e.target.value })} />
                  <datalist id={labelsId}>
                    {(projectLabels.data ?? []).map((l) => (
                      <option key={l} value={l} />
                    ))}
                  </datalist>
                </label>
                {(versions.data ?? []).some((v) => v.state !== "archived" || draft.fixVersions.includes(v.id)) && (
                  <fieldset className="fix-versions">
                    <legend>{t("Fix versions")}</legend>
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
                  {t("Story points")}
                  <input type="number" min={0} max={999} step={0.5} value={draft.estimate} onChange={(e) => setDraft({ ...draft, estimate: e.target.value })} />
                </label>
                <label className="check">
                  <input type="checkbox" checked={draft.confidential} onChange={(e) => setDraft({ ...draft, confidential: e.target.checked })} />
                  {t("Confidential — only admins and the people on this issue see it")}
                </label>
                <div className="row end">
                  {notice && (
                    <span role={notice.kind === "error" ? "alert" : "status"} className={notice.kind === "error" ? "error" : "muted"}>
                      {notice.text}
                    </span>
                  )}
                  <button type="submit" form={formId} disabled={save.isPending}>
                    {t("Save")}
                  </button>
                </div>
              </div>
            )}
            <IssueWatchers issueKey={issueKey} />
            {loaded && <IssueTimeTracking key={loaded.version} issue={loaded} />}
            {extensions}
          </aside>
        </div>
      </div>
    </div>
  );
}

function Comments({ issueKey, projectKey }: { issueKey: string; projectKey: string }) {
  const t = useT();
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
    <section className="comments" aria-label={t("Comments")}>
      <h3>{t("Comments")}</h3>
      <ul>
        {comments.data?.map((c) => (
          <li key={c.id}>
            <div className="comment-meta">
              <strong>{c.author_name}</strong> <time dateTime={c.created_at}>{new Date(c.created_at).toLocaleString()}</time>
            </div>
            <p className="comment-body">{c.body}</p>
            <CommentReactions issueKey={issueKey} comment={c} />
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
          label={t("Add a comment")}
          value={body}
          onChange={setBody}
          members={members.data ?? []}
          placeholder={t("Write a comment. Type @ to mention someone")}
        />
        {add.isError && <p role="alert" className="error">{add.error.message}</p>}
        <button type="submit" disabled={add.isPending}>
          {t("Comment")}
        </button>
      </form>
    </section>
  );
}
