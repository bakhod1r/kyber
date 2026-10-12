import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { type SprintReportIssue, api } from "../api";
import { StatTile, fmt } from "./charts/charts";

/** SprintReport: Jira's sprint report — committed, added, completed, not completed and removed issues. */
export function SprintReport({ projectKey }: { projectKey: string }) {
  const sprints = useQuery({ queryKey: ["sprints", projectKey], queryFn: () => api.sprints(projectKey) });
  const started = (sprints.data ?? []).filter((s) => s.state !== "planned");
  const [picked, setPicked] = useState("");
  const id = picked || (started.find((s) => s.state === "active") ?? started[started.length - 1])?.id;
  const report = useQuery({ queryKey: ["reports", "sprint", id], queryFn: () => api.sprintReport(id!), enabled: !!id });

  if (sprints.isSuccess && started.length === 0) {
    return (
      <section className="panel" aria-label="Sprint report">
        <h3>Sprint report</h3>
        <p className="muted">Start a sprint to see its report.</p>
      </section>
    );
  }
  const r = report.data;
  return (
    <section className="panel sprint-report" aria-label="Sprint report">
      <div className="row">
        <h3>Sprint report</h3>
        <label>
          Sprint
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
            <StatTile label="Committed" value={`${fmt(r.committed_points)} pts`} hint={`${r.committed_issues} issues at the start`} />
            <StatTile label="Added after start" value={`${fmt(r.added_points)} pts`} hint={`${r.added_issues} issues`} />
            <StatTile label="Completed" value={`${fmt(r.completed_points)} pts`} hint={`${r.completed.length} issues`} />
          </div>
          <IssueList title="Completed issues" items={r.completed} />
          <IssueList title={r.sprint.state === "closed" ? "Not completed (carried over)" : "Not completed yet"} items={r.not_completed} />
          <IssueList title="Removed from the sprint" items={r.removed} />
          <p className="muted">* added after the sprint started</p>
        </>
      )}
    </section>
  );
}

function IssueList({ title, items }: { title: string; items: SprintReportIssue[] }) {
  return (
    <div>
      <h4>
        {title} <span className="count">{items.length}</span>
      </h4>
      {items.length === 0 ? (
        <p className="muted">None.</p>
      ) : (
        <ul aria-label={title} className="child-list">
          {items.map((i) => (
            <li key={i.key}>
              <span className="key">{i.key}</span> <span>{i.title}</span>
              {i.added_after_start && <span title="Added after the sprint started"> *</span>}
              <span className="count">{fmt(i.points)} pts</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
