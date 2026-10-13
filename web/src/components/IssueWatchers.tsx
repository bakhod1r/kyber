import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";

/** IssueWatchers: Jira's watch toggle — watchers hear about comments and status changes. */
export function IssueWatchers({ issueKey }: { issueKey: string }) {
  const qc = useQueryClient();
  const key = ["watchers", issueKey];
  const watchers = useQuery({ queryKey: key, queryFn: () => api.watchers(issueKey) });
  const toggle = useMutation({
    mutationFn: (on: boolean) => (on ? api.watch(issueKey) : api.unwatch(issueKey)),
    onSettled: () => qc.invalidateQueries({ queryKey: key }),
  });
  if (!watchers.data) return null;
  const { watching, items } = watchers.data;
  return (
    <section className="watchers row" aria-label="Watchers">
      <button type="button" className="ghost" aria-pressed={watching} disabled={toggle.isPending} onClick={() => toggle.mutate(!watching)}>
        {watching ? "Stop watching" : "Watch"}
      </button>
      <span className="muted" title={items.map((w) => w.name).join(", ")}>
        {items.length} {items.length === 1 ? "watcher" : "watchers"}
      </span>
      {toggle.isError && (
        <span role="alert" className="error">
          {toggle.error.message}
        </span>
      )}
    </section>
  );
}
