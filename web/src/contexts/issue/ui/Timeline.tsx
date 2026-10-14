import { useQuery } from "@tanstack/react-query";
import { type Issue, issueUseCases, planTimeline } from "../application";
import { useT } from "../../../shared/i18n/i18n";

const label = (is: Issue) =>
  is.start_date && is.due_date && is.start_date !== is.due_date ? `${is.start_date} → ${is.due_date}` : (is.due_date ?? is.start_date ?? "");

/** Timeline: the project's epics and issues as bars over their start and due dates (Jira's Timeline). */
export function Timeline({ projectKey, onOpen }: { projectKey: string; onOpen: (key: string) => void }) {
  const t = useT();
  const issues = useQuery({ queryKey: ["issues", projectKey, "all"], queryFn: () => issueUseCases.issues(projectKey) });
  if (issues.isError) return <p role="alert" className="error">{(issues.error as Error).message}</p>;
  if (!issues.data) return <p className="muted">{t("Loading…")}</p>;
  const plan = planTimeline(issues.data);
  const pct = (n: number) => `${(n / plan.days) * 100}%`;
  return (
    <section className="timeline" aria-label={t("Timeline")}>
      <h2>{t("Timeline")}</h2>
      {plan.rows.length === 0 ? (
        <p className="muted">{t("Nothing is scheduled yet. Give issues a start or due date to see them here.")}</p>
      ) : (
        <>
          <p className="muted">
            {plan.from} · {(plan.days === 1 ? t("{n} day", { n: 1 }) : t("{n} days", { n: plan.days }))}
          </p>
          <ul className="timeline-rows">
            {plan.rows.map((r) => (
              <li key={r.issue.key} className={`timeline-row depth-${r.depth}`}>
                <button type="button" className="ghost timeline-open" onClick={() => onOpen(r.issue.key)}>
                  <span className="key">{r.issue.key}</span> {r.issue.title}
                </button>
                <div className="timeline-track">
                  {r.offset !== null && r.span !== null ? (
                    <div
                      className={`timeline-bar type-${r.issue.type}`}
                      role="img"
                      aria-label={`${r.issue.key}: ${label(r.issue)}`}
                      title={label(r.issue)}
                      style={{ marginLeft: pct(r.offset), width: pct(r.span) }}
                    />
                  ) : (
                    <span className="muted">{t("Not scheduled")}</span>
                  )}
                </div>
              </li>
            ))}
          </ul>
        </>
      )}
      {plan.unscheduled.length > 0 && (
        <details>
          <summary>{t("Unscheduled ({n})", { n: plan.unscheduled.length })}</summary>
          <ul>
            {plan.unscheduled.map((is) => (
              <li key={is.key}>
                <button type="button" className="ghost timeline-open" onClick={() => onOpen(is.key)}>
                  <span className="key">{is.key}</span> {is.title}
                </button>
              </li>
            ))}
          </ul>
        </details>
      )}
    </section>
  );
}
