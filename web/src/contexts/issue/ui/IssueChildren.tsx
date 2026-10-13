import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { issueUseCases, type Issue, type IssueType } from "../application";

/** IssueChildren: an epic's issues or an issue's sub-tasks, with Jira's progress bar (KYB-S45). */
export function IssueChildren({ issue, projectKey, onOpen }: { issue: Issue; projectKey: string; onOpen?: (key: string) => void }) {
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
  const heading = isEpic ? "Issues in this epic" : "Sub-tasks";
  const c = kids.data;
  const pct = c && c.total ? Math.round((c.done / c.total) * 100) : 0;
  return (
    <section className="children" aria-label={heading}>
      <h3>{heading}</h3>
      {kids.isError && <p className="error">{kids.error.message}</p>}
      {c && c.total > 0 && (
        <>
          <div className="progress" role="progressbar" aria-label={`${c.done} of ${c.total} done`} aria-valuemin={0} aria-valuemax={100} aria-valuenow={pct}>
            <span style={{ width: `${pct}%` }} />
          </div>
          {c.points > 0 && <p className="muted">{`${c.done_points} of ${c.points} points done`}</p>}
        </>
      )}
      {c && c.total === 0 && <p className="muted">{isEpic ? "No issues in this epic yet." : "No sub-tasks yet."}</p>}
      {c && c.items.length > 0 && (
        <ul className="child-list">
          {c.items.map((k) => (
            <li key={k.id}>
              <button type="button" className="link" onClick={() => onOpen?.(k.key)}>
                <span className={`type type-${k.type}`}>{k.type}</span> {k.key} {k.title}
              </button>
              <span className={`status status-${k.status}`}>{k.status.replace("_", " ")}</span>
            </li>
          ))}
        </ul>
      )}
      <form className="inline-form" onSubmit={submit}>
        <input aria-label="New child title" placeholder={isEpic ? "Add a story to this epic" : "Add a sub-task"} value={title} onChange={(e) => setTitle(e.target.value)} />
        <button type="submit" disabled={add.isPending}>
          Add
        </button>
      </form>
      {add.isError && <p role="alert" className="error">{add.error.message}</p>}
    </section>
  );
}
