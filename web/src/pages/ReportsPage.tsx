import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "../api";
import { BarList, GroupedBars, LineChart, StatTile, fmt } from "../components/charts/charts";

const STATUS: Record<string, string> = { todo: "To Do", in_progress: "In Progress", done: "Done" };
const title = (k: string) => STATUS[k] ?? k.charAt(0).toUpperCase() + k.slice(1);

export function Reports({ projectKey }: { projectKey: string }) {
  const [days, setDays] = useState(30);
  const summary = useQuery({ queryKey: ["reports", projectKey, "summary"], queryFn: () => api.reportSummary(projectKey) });
  const cvr = useQuery({
    queryKey: ["reports", projectKey, "cvr", days],
    queryFn: () => api.reportCreatedVsResolved(projectKey, days),
    placeholderData: (prev) => prev, // refetch keeps the frame
  });
  const sprints = useQuery({ queryKey: ["sprints", projectKey], queryFn: () => api.sprints(projectKey) });
  const focus = sprints.data?.find((s) => s.state === "active") ?? sprints.data?.find((s) => s.state === "closed");
  const burndown = useQuery({ queryKey: ["reports", "burndown", focus?.id], queryFn: () => api.burndown(focus!.id), enabled: !!focus });
  const velocity = useQuery({ queryKey: ["reports", projectKey, "velocity"], queryFn: () => api.reportVelocity(projectKey) });

  if (summary.isPending) return <p className="muted">Loading reports…</p>;
  if (summary.isError) return <p role="alert" className="error">{summary.error.message}</p>;
  const s = summary.data;
  const cycleDays = s.cycle_time.count ? `${(s.cycle_time.median_hours / 24).toFixed(1)} days` : "—";

  return (
    <div className="reports viz-root">
      <div className="filters">
        <label>
          Date range
          <select value={days} onChange={(e) => setDays(Number(e.target.value))}>
            <option value={7}>Last 7 days</option>
            <option value={30}>Last 30 days</option>
            <option value={90}>Last 90 days</option>
          </select>
        </label>
      </div>

      <section aria-label="Key figures" className="stats">
        <StatTile label="Open issues" value={fmt(s.open)} hint={`of ${fmt(s.total)} total`} />
        <StatTile label="Done" value={fmt(s.done)} hint={s.total ? `${Math.round((s.done / s.total) * 100)}% complete` : undefined} />
        <StatTile label="Open story points" value={fmt(s.open_points)} hint={`of ${fmt(s.total_points)} estimated`} />
        <StatTile label="Unassigned" value={fmt(s.unassigned)} />
        <StatTile label="Median cycle time" value={cycleDays} hint={s.cycle_time.count ? `${s.cycle_time.count} issues, last 30 days` : "no completed issues yet"} />
      </section>

      <div className="charts-grid">
        <BarList title="Issues by status" items={s.by_status.map((c) => ({ label: title(c.key), value: c.count, extra: c.points }))} extraLabel="Points" />
        <BarList title="Issues by type" items={s.by_type.map((c) => ({ label: title(c.key), value: c.count, extra: c.points }))} extraLabel="Points" />
        <BarList title="Issues by priority" items={s.by_priority.map((c) => ({ label: title(c.key), value: c.count, extra: c.points }))} extraLabel="Points" />
        <BarList title="Workload (open issues)" items={s.workload.map((w) => ({ label: w.name, value: w.count, extra: w.points }))} extraLabel="Points" />
      </div>

      {cvr.data && (
        <LineChart
          title="Created vs resolved"
          yLabel="cumulative issues"
          caption={`Cumulative issues created and resolved over the last ${days} days. A widening gap means work arrives faster than it is finished.`}
          series={[
            { name: "Created", slot: 1, points: cvr.data.days.map((d) => ({ x: new Date(`${d.date}T00:00:00Z`), y: d.cum_created })) },
            { name: "Resolved", slot: 2, points: cvr.data.days.map((d) => ({ x: new Date(`${d.date}T00:00:00Z`), y: d.cum_resolved })) },
          ]}
        />
      )}

      {!focus && sprints.isSuccess && (
        <section className="chart empty">
          <h3>Burndown</h3>
          <p className="muted">Start a sprint to see its burndown.</p>
        </section>
      )}
      {burndown.data && burndown.data.samples.length > 0 && (
        <LineChart
          title={`Burndown — ${burndown.data.sprint.name}`}
          yLabel="remaining story points"
          xMax={burndown.data.sprint.ends_at ? new Date(burndown.data.sprint.ends_at) : undefined}
          caption={
            <>
              Remaining story points from {fmt(burndown.data.start_points)} at the start ({burndown.data.start_issues} issues).{" "}
              Scope added after start: {fmt(burndown.data.added_points)} points ({burndown.data.added_issues} {burndown.data.added_issues === 1 ? "issue" : "issues"}).
              {burndown.data.removed_issues > 0 && ` Removed: ${burndown.data.removed_issues}.`}
            </>
          }
          series={[
            { name: "Remaining", slot: 1, step: true, points: burndown.data.samples.map((p) => ({ x: new Date(p.at), y: p.remaining_points })) },
            { name: "Ideal", slot: "muted", points: burndown.data.ideal.map((p) => ({ x: new Date(p.at), y: p.remaining_points })) },
          ]}
        />
      )}

      {velocity.data && velocity.data.sprints.length === 0 && (
        <section className="chart empty">
          <h3>Velocity</h3>
          <p className="muted">Complete a sprint to see velocity.</p>
        </section>
      )}
      {velocity.data && velocity.data.sprints.length > 0 && (
        <GroupedBars
          title="Velocity"
          categories={velocity.data.sprints.map((v) => v.name)}
          series={[
            { name: "Committed", slot: 1, values: velocity.data.sprints.map((v) => v.committed_points) },
            { name: "Completed", slot: 2, values: velocity.data.sprints.map((v) => v.completed_points) },
          ]}
        />
      )}
    </div>
  );
}
