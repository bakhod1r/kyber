import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { SprintReport } from "./SprintReport";
import { BarList, GroupedBars, LineChart, StatTile, fmt } from "../../../shared/ui/charts";
import { agileUseCases } from "../../agile/application";
import { insightsUseCases } from "../application";
import { issueUseCases } from "../../issue/application";
import { msg, type MessageKey, type Translate, useT } from "../../../shared/i18n/i18n";

const STATUS: Record<string, MessageKey> = { todo: msg("To Do"), in_progress: msg("In Progress"), done: msg("Done") };
const title = (t: Translate, k: string) => { const s = STATUS[k]; return s ? t(s) : k.charAt(0).toUpperCase() + k.slice(1); };

export function Reports({ projectKey }: { projectKey: string }) {
  const t = useT();
  const [days, setDays] = useState(30);
  const summary = useQuery({ queryKey: ["reports", projectKey, "summary"], queryFn: () => insightsUseCases.reportSummary(projectKey) });
  const cvr = useQuery({
    queryKey: ["reports", projectKey, "cvr", days],
    queryFn: () => insightsUseCases.reportCreatedVsResolved(projectKey, days),
    placeholderData: (prev) => prev, // refetch keeps the frame
  });
  const sprints = useQuery({ queryKey: ["sprints", projectKey], queryFn: () => agileUseCases.sprints(projectKey) });
  const focus = sprints.data?.find((s) => s.state === "active") ?? sprints.data?.find((s) => s.state === "closed");
  const burndown = useQuery({ queryKey: ["reports", "burndown", focus?.id], queryFn: () => insightsUseCases.burndown(focus!.id), enabled: !!focus });
  const workflow = useQuery({ queryKey: ["workflow", projectKey], queryFn: () => issueUseCases.workflow(projectKey) });
  const statusName = (k: string) => workflow.data?.statuses.find((st) => st.id === k)?.name ?? title(t, k);
  const control = useQuery({
    queryKey: ["reports", projectKey, "control", days],
    queryFn: () => insightsUseCases.reportControlChart(projectKey, days),
    placeholderData: (prev) => prev,
  });
  const velocity = useQuery({ queryKey: ["reports", projectKey, "velocity"], queryFn: () => insightsUseCases.reportVelocity(projectKey) });

  if (summary.isPending) return <p className="muted">{t("Loading reports…")}</p>;
  if (summary.isError) return <p role="alert" className="error">{summary.error.message}</p>;
  const s = summary.data;
  const cycleDays = s.cycle_time.count ? t("{n} days", { n: (s.cycle_time.median_hours / 24).toFixed(1) }) : "—";

  return (
    <div className="reports viz-root">
      <div className="filters">
        <label>
          {t("Date range")}
          <select value={days} onChange={(e) => setDays(Number(e.target.value))}>
            <option value={7}>{t("Last 7 days")}</option>
            <option value={30}>{t("Last 30 days")}</option>
            <option value={90}>{t("Last 90 days")}</option>
          </select>
        </label>
      </div>

      <section aria-label={t("Key figures")} className="stats">
        <StatTile label={t("Open issues")} value={fmt(s.open)} hint={t("of {n} total", { n: fmt(s.total) })} />
        <StatTile label={t("Done")} value={fmt(s.done)} hint={s.total ? t("{n}% complete", { n: Math.round((s.done / s.total) * 100) }) : undefined} />
        <StatTile label={t("Open story points")} value={fmt(s.open_points)} hint={t("of {n} estimated", { n: fmt(s.total_points) })} />
        <StatTile label={t("Unassigned")} value={fmt(s.unassigned)} />
        <StatTile label={t("Median cycle time")} value={cycleDays} hint={s.cycle_time.count ? t("{n} issues, last 30 days", { n: s.cycle_time.count }) : t("no completed issues yet")} />
      </section>

      <div className="charts-grid">
        <BarList title={t("Issues by status")} items={s.by_status.map((c) => ({ label: statusName(c.key), value: c.count, extra: c.points }))} extraLabel={t("Points")} />
        <BarList title={t("Issues by type")} items={s.by_type.map((c) => ({ label: title(t, c.key), value: c.count, extra: c.points }))} extraLabel={t("Points")} />
        <BarList title={t("Issues by priority")} items={s.by_priority.map((c) => ({ label: title(t, c.key), value: c.count, extra: c.points }))} extraLabel={t("Points")} />
        <BarList title={t("Workload (open issues)")} items={s.workload.map((w) => ({ label: w.name, value: w.count, extra: w.points }))} extraLabel={t("Points")} />
      </div>

      {cvr.data && (
        <LineChart
          title={t("Created vs resolved")}
          yLabel={t("cumulative issues")}
          caption={t("Cumulative issues created and resolved over the last {n} days. A widening gap means work arrives faster than it is finished.", { n: days })}
          series={[
            { name: t("Created"), slot: 1, points: cvr.data.days.map((d) => ({ x: new Date(`${d.date}T00:00:00Z`), y: d.cum_created })) },
            { name: t("Resolved"), slot: 2, points: cvr.data.days.map((d) => ({ x: new Date(`${d.date}T00:00:00Z`), y: d.cum_resolved })) },
          ]}
        />
      )}

      {!focus && sprints.isSuccess && (
        <section className="chart empty">
          <h3>{t("Burndown")}</h3>
          <p className="muted">{t("Start a sprint to see its burndown.")}</p>
        </section>
      )}
      {burndown.data && burndown.data.samples.length > 0 && (
        <LineChart
          title={t("Burndown — {name}", { name: burndown.data.sprint.name })}
          yLabel={t("remaining story points")}
          xMax={burndown.data.sprint.ends_at ? new Date(burndown.data.sprint.ends_at) : undefined}
          caption={
            <>
              {t("Remaining story points from {points} at the start ({n} issues).", { points: fmt(burndown.data.start_points), n: burndown.data.start_issues })}{" "}
              {burndown.data.added_issues === 1
                ? t("Scope added after start: {points} points ({n} issue).", { points: fmt(burndown.data.added_points), n: 1 })
                : t("Scope added after start: {points} points ({n} issues).", { points: fmt(burndown.data.added_points), n: burndown.data.added_issues })}
              {burndown.data.removed_issues > 0 && ` ${t("Removed: {n}.", { n: burndown.data.removed_issues })}`}
            </>
          }
          series={[
            { name: t("Remaining"), slot: 1, step: true, points: burndown.data.samples.map((p) => ({ x: new Date(p.at), y: p.remaining_points })) },
            { name: t("Ideal"), slot: "muted", points: burndown.data.ideal.map((p) => ({ x: new Date(p.at), y: p.remaining_points })) },
          ]}
        />
      )}

      {control.data && control.data.points.length === 0 && (
        <section className="chart empty">
          <h3>{t("Control chart")}</h3>
          <p className="muted">{t("Finish an issue to see its cycle time.")}</p>
        </section>
      )}
      {control.data && control.data.points.length > 0 && (
        <LineChart
          title={t("Control chart")}
          yLabel={t("cycle time (hours)")}
          caption={t("Cycle time from first In Progress to Done for {n} issues. Average {avg} days, standard deviation {sd} days.", {
            n: control.data.points.length,
            avg: (control.data.average_hours / 24).toFixed(1),
            sd: (control.data.stddev_hours / 24).toFixed(1),
          })}
          series={[
            { name: t("Issues"), slot: 1, dots: true, points: control.data.points.map((p) => ({ x: new Date(p.done_at), y: p.cycle_hours })) },
            { name: t("Rolling average"), slot: 2, points: control.data.points.map((p) => ({ x: new Date(p.done_at), y: p.rolling_hours })) },
          ]}
        />
      )}

      <SprintReport projectKey={projectKey} />
      {velocity.data && velocity.data.sprints.length === 0 && (
        <section className="chart empty">
          <h3>{t("Velocity")}</h3>
          <p className="muted">{t("Complete a sprint to see velocity.")}</p>
        </section>
      )}
      {velocity.data && velocity.data.sprints.length > 0 && (
        <GroupedBars
          title={t("Velocity")}
          categories={velocity.data.sprints.map((v) => v.name)}
          series={[
            { name: t("Committed"), slot: 1, values: velocity.data.sprints.map((v) => v.committed_points) },
            { name: t("Completed"), slot: 2, values: velocity.data.sprints.map((v) => v.completed_points) },
          ]}
        />
      )}
    </div>
  );
}
