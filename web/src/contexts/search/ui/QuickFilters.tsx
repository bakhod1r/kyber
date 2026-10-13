import { useQuery } from "@tanstack/react-query";
import { useEffect } from "react";
import { projectQuery, QUICK_FILTERS, searchUseCases } from "../application";

export type QuickFilterState = { quick: string[]; saved: string };

/** QuickFilters: Jira's quick filters (and saved filters) for a project view. It reports the keys
 * of the issues that match, or null while no filter is on. */
export function QuickFilters({
  projectKey,
  value,
  onChange,
  onKeys,
}: {
  projectKey: string;
  value: QuickFilterState;
  onChange: (v: QuickFilterState) => void;
  onKeys: (keys: Set<string> | null) => void;
}) {
  const filters = useQuery({ queryKey: ["filters"], queryFn: searchUseCases.filters });
  const active = value.quick.length > 0 || value.saved !== "";
  const jql = projectQuery(projectKey, value.quick, value.saved);
  const matches = useQuery({ queryKey: ["search", jql], queryFn: () => searchUseCases.search(jql), enabled: active });
  useEffect(() => {
    onKeys(active && matches.data ? new Set(matches.data.items.map((i) => i.key)) : null);
  }, [active, matches.data, onKeys]);

  const toggle = (id: string) =>
    onChange({ ...value, quick: value.quick.includes(id) ? value.quick.filter((q) => q !== id) : [...value.quick, id] });
  return (
    <div className="quick-filters row" role="group" aria-label="Quick filters">
      {QUICK_FILTERS.map((f) => (
        <button key={f.id} type="button" className="ghost chip" aria-pressed={value.quick.includes(f.id)} onClick={() => toggle(f.id)}>
          {f.label}
        </button>
      ))}
      {(filters.data ?? []).length > 0 && (
        <label>
          Saved filter
          <select value={value.saved} onChange={(e) => onChange({ ...value, saved: e.target.value })}>
            <option value="">None</option>
            {filters.data?.map((f) => (
              <option key={f.id} value={f.jql}>
                {f.name}
              </option>
            ))}
          </select>
        </label>
      )}
      {active && (
        <button type="button" className="ghost" onClick={() => onChange({ quick: [], saved: "" })}>
          Clear filters
        </button>
      )}
      {matches.isError && <span role="alert" className="error">{matches.error.message}</span>}
    </div>
  );
}
