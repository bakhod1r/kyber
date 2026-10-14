import { useQuery } from "@tanstack/react-query";
import { issueUseCases, DEFAULT_WORKFLOW, type HistoryEntry, RESOLUTIONS } from "../application";
import { msg, type MessageKey, useT } from "../../../shared/i18n/i18n";

const FIELD: Record<string, MessageKey> = {
  title: msg("Summary"), description: msg("Description"), type: msg("Type"), status: msg("Status"), priority: msg("Priority"),
  assignee: msg("Assignee"), estimate: msg("Story points"), sprint: msg("Sprint"), rank: msg("Rank"), resolution: msg("Resolution"), original_estimate: msg("Original estimate"), remaining_estimate: msg("Remaining estimate"), time_spent: msg("Time spent"), fix_versions: msg("Fix versions"),
  security: msg("Security level"), parent: msg("Parent"),
};

/** IssueHistory is Jira's History tab: every action on the issue, oldest first. */
export function IssueHistory({ issueKey, projectKey }: { issueKey: string; projectKey: string }) {
  const t = useT();
  // The history is projected asynchronously from the event log, so it refreshes while open.
  const history = useQuery({ queryKey: ["history", issueKey], queryFn: () => issueUseCases.history(issueKey), refetchInterval: 5000 });
  const workflow = useQuery({ queryKey: ["workflow", projectKey], queryFn: () => issueUseCases.workflow(projectKey) });
  const status = (id: string) => (workflow.data ?? DEFAULT_WORKFLOW).statuses.find((s) => s.id === id)?.name ?? id;
  const value = (e: HistoryEntry, raw: string, name: string) => {
    if (raw === "") return t("None");
    if (e.field === "resolution") return RESOLUTIONS.find((r) => r.id === raw)?.label ?? raw;
    return e.field === "status" ? status(raw) : name || raw;
  };
  const line = (e: HistoryEntry) => {
    const who = e.by_name || t("System");
    const field = (k: string) => { const f = FIELD[k]; return f ? t(f) : k.replace(/^custom:/, ""); };
    switch (e.field) {
      case "created":
        return t("{who} created the issue", { who });
      case "imported":
        return e.to ? t("{who} imported the issue from {from}", { who, from: e.to }) : t("{who} imported the issue", { who });
      case "comment":
        return t("{who} commented: {text}", { who, text: e.to });
      case "description":
      case "rank":
        return t("{who} changed {field}", { who, field: field(e.field) });
    }
    const label = field(e.field);
    if (e.from === "" && e.to === "") return t("{who} changed {field}", { who, field: label }); // recovered history: values unknown
    return t("{who} changed {field}: {from} → {to}", { who, field: label, from: value(e, e.from, e.from_name), to: value(e, e.to, e.to_name) });
  };

  return (
    <section className="history" aria-label={t("Issue history")}>
      <h3>{t("History")}</h3>
      {history.isPending && <p className="muted">{t("Loading history…")}</p>}
      {history.isError && <p className="error">{history.error.message}</p>}
      {history.data && (
        <ol aria-label={t("History")} className="history-list">
          {history.data.map((e, i) => (
            <li key={`${e.at}-${i}`}>
              <span>{line(e)}</span>{" "}
              <time className="muted" dateTime={e.at}>
                {new Date(e.at).toLocaleString()}
              </time>
            </li>
          ))}
        </ol>
      )}
    </section>
  );
}
