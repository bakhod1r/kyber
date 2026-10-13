import { useQuery } from "@tanstack/react-query";
import { agileUseCases } from "../../agile/application";
import { BarList, StatTile, fmt } from "../../../shared/ui/charts";
import { insightsUseCases } from "../application";

const CATEGORY: Record<string, string> = { todo: "To Do", in_progress: "In Progress", done: "Done" };

/** ProjectSummary: Jira's Summary tab — the project at a glance. */
export function ProjectSummary({ projectKey }: { projectKey: string }) {
  const summary = useQuery({ queryKey: ["reports", projectKey, "summary"], queryFn: () => insightsUseCases.reportSummary(projectKey) });
  const week = useQuery({ queryKey: ["reports", projectKey, "cvr", 7], queryFn: () => insightsUseCases.reportCreatedVsResolved(projectKey, 7) });
  const versions = useQuery({ queryKey: ["versions", projectKey], queryFn: () => agileUseCases.versions(projectKey) });

  if (summary.isPending) return <p className="muted">Loading summary…</p>;
  if (summary.isError) return <p role="alert" className="error">{summary.error.message}</p>;
  const s = summary.data;
  const created = (week.data?.days ?? []).reduce((n, d) => n + d.created, 0);
  const resolved = (week.data?.days ?? []).reduce((n, d) => n + d.resolved, 0);
  const unreleased = (versions.data ?? []).filter((v) => v.state === "unreleased");
  return (
    <div className="summary viz-root" aria-label="Project summary">
      <section aria-label="Last 7 days" className="stats">
        <StatTile label="Created" value={fmt(created)} hint="in the last 7 days" />
        <StatTile label="Resolved" value={fmt(resolved)} hint="in the last 7 days" />
        <StatTile label="Open" value={fmt(s.open)} hint={`of ${fmt(s.total)} issues`} />
        <StatTile label="Unassigned" value={fmt(s.unassigned)} />
      </section>
      <div className="charts-grid">
        <BarList title="Status overview" items={s.by_status.map((c) => ({ label: CATEGORY[c.key] ?? c.key, value: c.count }))} />
        <BarList title="Types of work" items={s.by_type.map((c) => ({ label: c.key, value: c.count }))} />
        <BarList title="Priority breakdown" items={s.by_priority.map((c) => ({ label: c.key, value: c.count }))} />
        <BarList title="Team workload" items={s.workload.map((w) => ({ label: w.name, value: w.count }))} />
      </div>
      <section className="panel" aria-label="Upcoming releases">
        <h3>Upcoming releases</h3>
        {unreleased.length === 0 ? (
          <p className="muted">No unreleased versions.</p>
        ) : (
          <ul className="child-list">
            {unreleased.map((v) => (
              <li key={v.id}>
                <span>{v.name}</span>
                <meter min={0} max={Math.max(v.issues ?? 0, 1)} value={v.done ?? 0} aria-label={`${v.name} progress`} />
                <span className="muted">
                  {v.done ?? 0}/{v.issues ?? 0} done{v.release_date ? ` · ${new Date(v.release_date).toLocaleDateString()}` : ""}
                </span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
