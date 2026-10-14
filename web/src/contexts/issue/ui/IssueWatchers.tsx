import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { issueUseCases } from "../application";
import { useT } from "../../../shared/i18n/i18n";

/** IssueWatchers: Jira's watch toggle — watchers hear about comments and status changes. */
export function IssueWatchers({ issueKey }: { issueKey: string }) {
  const t = useT();
  const qc = useQueryClient();
  const key = ["watchers", issueKey];
  const watchers = useQuery({ queryKey: key, queryFn: () => issueUseCases.watchers(issueKey) });
  const toggle = useMutation({
    mutationFn: (on: boolean) => (on ? issueUseCases.watch(issueKey) : issueUseCases.unwatch(issueKey)),
    onSettled: () => qc.invalidateQueries({ queryKey: key }),
  });
  if (!watchers.data) return null;
  const { watching, items } = watchers.data;
  return (
    <section className="watchers row" aria-label={t("Watchers")}>
      <button type="button" className="ghost" aria-pressed={watching} disabled={toggle.isPending} onClick={() => toggle.mutate(!watching)}>
        {watching ? t("Stop watching") : t("Watch")}
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
