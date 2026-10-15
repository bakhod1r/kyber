import { typeLabel } from "../../../shared/i18n/labels";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { issueUseCases, type Issue, type IssueType } from "../application";
import { useT } from "../../../shared/i18n/i18n";

/** IssueChildren: an epic's issues or an issue's sub-tasks, with Jira's progress bar (KYB-S45). */
export function IssueChildren({ issue, projectKey, onOpen }: { issue: Issue; projectKey: string; onOpen?: (key: string) => void }) {
  const t = useT();
  const qc = useQueryClient();
  const isEpic = issue.type === "epic";
  const childType: IssueType = isEpic ? "story" : "subtask";
  const enabled = issue.type !== "subtask";
  const kids = useQuery({ queryKey: ["children", issue.key], queryFn: () => issueUseCases.children(issue.key), enabled });
  const [title, setTitle] = useState("");
  const add = useMutation({
    mutationFn: () => issueUseCases.createIssue(projectKey, title, childType, issue.key),
    onSuccess: () => {
      setTitle("");
      return Promise.all([qc.invalidateQueries({ queryKey: ["children", issue.key] }), qc.invalidateQueries({ queryKey: ["issues", projectKey] })]);
    },
  });
  if (!enabled) return null;
  function submit(e: FormEvent) {
    e.preventDefault();
    if (title.trim()) add.mutate();
  }
  const heading = isEpic ? t("Issues in this epic") : t("Sub-tasks");
  const c = kids.data;
  const pct = c && c.total ? Math.round((c.done / c.total) * 100) : 0;
  return (
    <section className="children" aria-label={heading}>
      <h3>{heading}</h3>
      {kids.isError && <p className="error">{kids.error.message}</p>}
      {c && c.total > 0 && (
        <>
          <div className="progress" role="progressbar" aria-label={t("{done} of {total} done", { done: c.done, total: c.total })} aria-valuemin={0} aria-valuemax={100} aria-valuenow={pct}>
            <span style={{ width: `${pct}%` }} />
          </div>
          {c.points > 0 && <p className="muted">{t("{done} of {total} points done", { done: c.done_points, total: c.points })}</p>}
        </>
      )}
      {c && c.total === 0 && <p className="muted">{isEpic ? t("No issues in this epic yet.") : t("No sub-tasks yet.")}</p>}
      {c && c.items.length > 0 && (
        <ul className="child-list">
          {c.items.map((k) => (
            <li key={k.id}>
              <button type="button" className="link" onClick={() => onOpen?.(k.key)}>
                <span className={`type type-${k.type}`}>{typeLabel(t, k.type)}</span> {k.key} {k.title}
              </button>
              <span className={`status status-${k.status}`}>{k.status.replace("_", " ")}</span>
            </li>
          ))}
        </ul>
      )}
      <form className="inline-form" onSubmit={submit}>
        <input aria-label={t("New child title")} placeholder={isEpic ? t("Add a story to this epic") : t("Add a sub-task")} value={title} onChange={(e) => setTitle(e.target.value)} />
        <button type="submit" disabled={add.isPending}>
          {t("Add")}
        </button>
      </form>
      {add.isError && <p role="alert" className="error">{add.error.message}</p>}
    </section>
  );
}
