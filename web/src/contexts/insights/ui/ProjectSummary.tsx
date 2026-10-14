import { useQuery } from "@tanstack/react-query";
import { agileUseCases } from "../../agile/application";
import { BarList, StatTile, fmt } from "../../../shared/ui/charts";
import { insightsUseCases } from "../application";
import { msg, type MessageKey, useT } from "../../../shared/i18n/i18n";

const CATEGORY: Record<string, MessageKey> = { todo: msg("To Do"), in_progress: msg("In Progress"), done: msg("Done") };

/** ProjectSummary: Jira's Summary tab — the project at a glance. */
export function ProjectSummary({ projectKey }: { projectKey: string }) {
  const t = useT();
  const label = (k: string) => {
    const m = CATEGORY[k];
    return m ? t(m) : k;
  };
  const summary = useQuery({ queryKey: ["reports", projectKey, "summary"], queryFn: () => insightsUseCases.reportSummary(projectKey) });
  const week = useQuery({ queryKey: ["reports", projectKey, "cvr", 7], queryFn: () => insightsUseCases.reportCreatedVsResolved(projectKey, 7) });
  const versions = useQuery({ queryKey: ["versions", projectKey], queryFn: () => agileUseCases.versions(projectKey) });

  if (summary.isPending) return <p className="muted">{t("Loading summary…")}</p>;
  if (summary.isError) return <p role="alert" className="error">{summary.error.message}</p>;
  const s = summary.data;
  const created = (week.data?.days ?? []).reduce((n, d) => n + d.created, 0);
  const resolved = (week.data?.days ?? []).reduce((n, d) => n + d.resolved, 0);
  const unreleased = (versions.data ?? []).filter((v) => v.state === "unreleased");
  return (
    <div className="summary viz-root" aria-label={t("Project summary")}>
      <section aria-label={t("Last 7 days")} className="stats">
        <StatTile label={t("Created")} value={fmt(created)} hint={t("in the last 7 days")} />
        <StatTile label={t("Resolved")} value={fmt(resolved)} hint={t("in the last 7 days")} />
        <StatTile label={t("Open")} value={fmt(s.open)} hint={t("of {n} issues", { n: fmt(s.total) })} />
        <StatTile label={t("Unassigned")} value={fmt(s.unassigned)} />
      </section>
      <div className="charts-grid">
        <BarList title={t("Status overview")} items={s.by_status.map((c) => ({ label: label(c.key), value: c.count }))} />
        <BarList title={t("Types of work")} items={s.by_type.map((c) => ({ label: c.key, value: c.count }))} />
        <BarList title={t("Priority breakdown")} items={s.by_priority.map((c) => ({ label: c.key, value: c.count }))} />
        <BarList title={t("Team workload")} items={s.workload.map((w) => ({ label: w.name, value: w.count }))} />
      </div>
      <section className="panel" aria-label={t("Upcoming releases")}>
        <h3>{t("Upcoming releases")}</h3>
        {unreleased.length === 0 ? (
          <p className="muted">{t("No unreleased versions.")}</p>
        ) : (
          <ul className="child-list">
            {unreleased.map((v) => (
              <li key={v.id}>
                <span>{v.name}</span>
                <meter min={0} max={Math.max(v.issues ?? 0, 1)} value={v.done ?? 0} aria-label={t("{name} progress", { name: v.name })} />
                <span className="muted">
                  {t("{done}/{total} done", { done: v.done ?? 0, total: v.issues ?? 0 })}{v.release_date ? ` · ${new Date(v.release_date).toLocaleDateString()}` : ""}
                </span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
