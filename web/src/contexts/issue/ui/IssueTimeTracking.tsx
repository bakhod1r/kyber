import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { type Issue, issueUseCases } from "../application";
import { useT } from "../../../shared/i18n/i18n";

/** IssueTimeTracking: Jira's time tracking — estimates, logged time and the worklog list. */
export function IssueTimeTracking({ issue }: { issue: Issue }) {
  const t = useT();
  const qc = useQueryClient();
  const key = issue.key;
  const worklogs = useQuery({ queryKey: ["worklogs", key], queryFn: () => issueUseCases.worklogs(key), refetchInterval: 5000 });
  const [estimate, setEstimate] = useState(issue.time_tracking?.original ?? "");
  const [spent, setSpent] = useState("");
  const [comment, setComment] = useState("");
  const [error, setError] = useState<string | null>(null);
  const refresh = () =>
    Promise.all([qc.invalidateQueries({ queryKey: ["issue", key] }), qc.invalidateQueries({ queryKey: ["worklogs", key] })]);
  const failed = (err: Error) => setError(err.message);

  const saveEstimate = useMutation({
    mutationFn: () => issueUseCases.editIssue(key, { version: issue.version, original_estimate: estimate.trim() }),
    onMutate: () => setError(null),
    onSuccess: (updated) => {
      qc.setQueryData(["issue", key], updated);
    },
    onError: failed,
  });
  const log = useMutation({
    mutationFn: () => issueUseCases.logWork(key, spent.trim(), comment),
    onMutate: () => setError(null),
    onSuccess: () => {
      setSpent("");
      setComment("");
      return refresh();
    },
    onError: failed,
  });
  const remove = useMutation({ mutationFn: (id: string) => issueUseCases.deleteWorklog(id), onSuccess: refresh, onError: failed });

  const tt = issue.time_tracking;
  const total = Math.max((tt?.spent_seconds ?? 0) + (tt?.remaining_seconds ?? 0), 1);
  function onLog(e: FormEvent) {
    e.preventDefault();
    if (spent.trim()) log.mutate();
  }
  return (
    <section className="time-tracking" aria-label={t("Time tracking")}>
      <h3>{t("Time tracking")}</h3>
      <meter min={0} max={total} value={tt?.spent_seconds ?? 0} aria-label={t("Time logged")} />
      <p className="muted">
        {t("Logged {spent} · Remaining {remaining} · Estimate {estimate}", { spent: tt?.spent || "0m", remaining: tt?.remaining || "0m", estimate: tt?.original || t("none") })}
      </p>
      <div className="row">
        <label>
          {t("Original estimate")}
          <input value={estimate} placeholder={t("e.g. 1d 4h")} onChange={(e) => setEstimate(e.target.value)} />
        </label>
        <button type="button" className="ghost" disabled={saveEstimate.isPending} onClick={() => saveEstimate.mutate()}>
          {t("Set estimate")}
        </button>
      </div>
      <form className="row" onSubmit={onLog}>
        <label>
          {t("Time spent")}
          <input value={spent} placeholder={t("e.g. 1h 30m")} onChange={(e) => setSpent(e.target.value)} />
        </label>
        <label>
          {t("Worklog comment")}
          <input value={comment} onChange={(e) => setComment(e.target.value)} maxLength={2000} />
        </label>
        <button type="submit" disabled={log.isPending || !spent.trim()}>
          {t("Log work")}
        </button>
      </form>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {(worklogs.data?.length ?? 0) > 0 && (
        <ul aria-label={t("Worklogs")} className="child-list">
          {worklogs.data?.map((w) => (
            <li key={w.id}>
              <span>
                <strong>{w.spent}</strong> {w.author_name} · {new Date(w.started).toLocaleDateString()}
                {w.comment && <> — {w.comment}</>}
              </span>
              <button type="button" className="ghost" aria-label={t("Delete worklog {spent} by {name}", { spent: w.spent, name: w.author_name })} onClick={() => remove.mutate(w.id)}>
                {t("Delete")}
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
