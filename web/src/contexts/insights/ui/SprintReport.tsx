import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { insightsUseCases, type SprintReportIssue } from "../application";
import { StatTile, fmt } from "../../../shared/ui/charts";
import { agileUseCases } from "../../agile/application";
import { useT } from "../../../shared/i18n/i18n";

/** SprintReport: Jira's sprint report — committed, added, completed, not completed and removed issues. */
export function SprintReport({ projectKey }: { projectKey: string }) {
  const t = useT();
  const sprints = useQuery({ queryKey: ["sprints", projectKey], queryFn: () => agileUseCases.sprints(projectKey) });
  const started = (sprints.data ?? []).filter((s) => s.state !== "planned");
  const [picked, setPicked] = useState("");
  const id = picked || (started.find((s) => s.state === "active") ?? started[started.length - 1])?.id;
  const report = useQuery({ queryKey: ["reports", "sprint", id], queryFn: () => insightsUseCases.sprintReport(id!), enabled: !!id });

  if (sprints.isSuccess && started.length === 0) {
    return (
      <section className="panel" aria-label={t("Sprint report")}>
        <h3>{t("Sprint report")}</h3>
        <p className="muted">{t("Start a sprint to see its report.")}</p>
      </section>
    );
  }
  const r = report.data;
  return (
    <section className="panel sprint-report" aria-label={t("Sprint report")}>
      <div className="row">
        <h3>{t("Sprint report")}</h3>
        <label>
          {t("Sprint")}
          <select value={id ?? ""} onChange={(e) => setPicked(e.target.value)}>
            {started.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name} ({s.state})
              </option>
            ))}
          </select>
        </label>
      </div>
      {report.isError && <p role="alert" className="error">{report.error.message}</p>}
      {r && (
        <>
          <div className="stats">
            <StatTile label={t("Committed")} value={t("{n} pts", { n: fmt(r.committed_points) })} hint={t("{n} issues at the start", { n: r.committed_issues })} />
            <StatTile label={t("Added after start")} value={t("{n} pts", { n: fmt(r.added_points) })} hint={t("{n} issues", { n: r.added_issues })} />
            <StatTile label={t("Completed")} value={t("{n} pts", { n: fmt(r.completed_points) })} hint={t("{n} issues", { n: r.completed.length })} />
            {(r.time_original_seconds ?? 0) + (r.time_spent_seconds ?? 0) > 0 && (
              <StatTile label={t("Time logged")} value={hours(r.time_spent_seconds ?? 0)} hint={t("estimate {estimate}, remaining {remaining}", { estimate: hours(r.time_original_seconds ?? 0), remaining: hours(r.time_remaining_seconds ?? 0) })} />
            )}
          </div>
          <IssueList title={t("Completed issues")} items={r.completed} />
          <IssueList title={r.sprint.state === "closed" ? t("Not completed (carried over)") : t("Not completed yet")} items={r.not_completed} />
          <IssueList title={t("Removed from the sprint")} items={r.removed} />
          <p className="muted">{t("* added after the sprint started")}</p>
        </>
      )}
    </section>
  );
}

function IssueList({ title, items }: { title: string; items: SprintReportIssue[] }) {
  const t = useT();
  return (
    <div>
      <h4>
        {title} <span className="count">{items.length}</span>
      </h4>
      {items.length === 0 ? (
        <p className="muted">{t("None.")}</p>
      ) : (
        <ul aria-label={title} className="child-list">
          {items.map((i) => (
            <li key={i.key}>
              <span className="key">{i.key}</span> <span>{i.title}</span>
              {i.added_after_start && <span title={t("Added after the sprint started")}> *</span>}
              <span className="count">{t("{n} pts", { n: fmt(i.points) })}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

const hours = (seconds: number) => `${fmt(Math.round((seconds / 3600) * 10) / 10)}h`;
