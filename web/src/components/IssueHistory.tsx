import { useQuery } from "@tanstack/react-query";
import { DEFAULT_WORKFLOW, type HistoryEntry, api } from "../api";

const FIELD: Record<string, string> = {
  title: "Summary", description: "Description", type: "Type", status: "Status", priority: "Priority",
  assignee: "Assignee", estimate: "Story points", sprint: "Sprint", rank: "Rank",
};

/** IssueHistory is Jira's History tab: every action on the issue, oldest first. */
export function IssueHistory({ issueKey, projectKey }: { issueKey: string; projectKey: string }) {
  // The history is projected asynchronously from the event log, so it refreshes while open.
  const history = useQuery({ queryKey: ["history", issueKey], queryFn: () => api.history(issueKey), refetchInterval: 5000 });
  const workflow = useQuery({ queryKey: ["workflow", projectKey], queryFn: () => api.workflow(projectKey) });
  const status = (id: string) => (workflow.data ?? DEFAULT_WORKFLOW).statuses.find((s) => s.id === id)?.name ?? id;
  const value = (e: HistoryEntry, raw: string, name: string) => {
    if (raw === "") return "None";
    return e.field === "status" ? status(raw) : name || raw;
  };
  const line = (e: HistoryEntry) => {
    const who = e.by_name || "System";
    switch (e.field) {
      case "created":
        return `${who} created the issue`;
      case "imported":
        return `${who} imported the issue${e.to ? ` from ${e.to}` : ""}`;
      case "comment":
        return `${who} commented: ${e.to}`;
      case "description":
      case "rank":
        return `${who} changed ${FIELD[e.field]}`;
    }
    const label = FIELD[e.field] ?? e.field;
    if (e.from === "" && e.to === "") return `${who} changed ${label}`; // recovered history: values unknown
    return `${who} changed ${label}: ${value(e, e.from, e.from_name)} → ${value(e, e.to, e.to_name)}`;
  };

  return (
    <section className="history" aria-label="Issue history">
      <h3>History</h3>
      {history.isPending && <p className="muted">Loading history…</p>}
      {history.isError && <p className="error">{history.error.message}</p>}
      {history.data && (
        <ol aria-label="History" className="history-list">
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
