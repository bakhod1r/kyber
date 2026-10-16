import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { projectQuery, searchUseCases } from "../application";
import { type QuickFilterState, QuickFilters } from "./QuickFilters";
import { msg, type MessageKey, useLabel, useT } from "../../../shared/i18n/i18n";

const COLUMNS: { field: string; label: MessageKey; jql: string }[] = [
  { field: "key", label: msg("Key"), jql: "key" },
  { field: "title", label: msg("Summary"), jql: "summary" },
  { field: "type", label: msg("Type"), jql: "type" },
  { field: "status", label: msg("Status"), jql: "status" },
  { field: "priority", label: msg("Priority"), jql: "priority" },
  { field: "points", label: msg("Points"), jql: '"story points"' },
];

/** IssueList: Jira's List view — every issue of the project in a sortable table, with quick filters. */
export function IssueList({ projectKey, onOpen }: { projectKey: string; onOpen?: (issueKey: string) => void }) {
  const t = useT();
  const label = useLabel();
  const [filters, setFilters] = useState<QuickFilterState>({ quick: [], saved: "" });
  const [sort, setSort] = useState<{ jql: string; desc: boolean }>({ jql: "key", desc: false });
  const jql = projectQuery(projectKey, filters.quick, filters.saved, `${sort.jql}${sort.desc ? " DESC" : ""}`);
  const result = useQuery({ queryKey: ["search", jql], queryFn: () => searchUseCases.search(jql) });
  const sortBy = (col: string) => setSort((s) => ({ jql: col, desc: s.jql === col ? !s.desc : false }));
  return (
    <section className="issue-list" aria-label={t("Issue list")}>
      <QuickFilters projectKey={projectKey} value={filters} onChange={setFilters} onKeys={noop} />
      {result.isError && <p role="alert" className="error">{result.error.message}</p>}
      {result.data && (
        <>
          <p className="muted">{result.data.total === 1 ? t("1 issue") : t("{n} issues", { n: result.data.total })}</p>
          <table className="versions-table">
            <thead>
              <tr>
                {COLUMNS.map((c) => (
                  <th key={c.field} scope="col" aria-sort={sort.jql === c.jql ? (sort.desc ? "descending" : "ascending") : "none"}>
                    <button type="button" className="ghost" onClick={() => sortBy(c.jql)}>
                      {t(c.label)}
                      {sort.jql === c.jql ? (sort.desc ? " ↓" : " ↑") : ""}
                    </button>
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {result.data.items.map((is) => (
                <tr key={is.key}>
                  <td>
                    <button type="button" className="ghost key" onClick={() => onOpen?.(is.key)}>
                      {is.key}
                    </button>
                  </td>
                  <td>{is.title}</td>
                  <td className="cap">{is.type}</td>
                  <td>{label(is.status_name || is.status)}</td>
                  <td className="cap">{is.priority}</td>
                  <td>{is.points ?? ""}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
    </section>
  );
}

function noop() {}
