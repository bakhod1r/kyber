import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { EXAMPLE_QUERIES, type Filter, searchUseCases } from "../application";
import { useT } from "../../../shared/i18n/i18n";

/** SearchPage: JQL search across the projects you can see, with saved filters (Jira's issue navigator). */
export function SearchPage() {
  const t = useT();
  const qc = useQueryClient();
  const [params, setParams] = useSearchParams();
  const jql = params.get("jql") ?? "";
  const [draft, setDraft] = useState(jql);
  const [name, setName] = useState("");
  const [shared, setShared] = useState(false);
  const [notice, setNotice] = useState<{ kind: "ok" | "error"; text: string } | null>(null);
  const results = useQuery({ queryKey: ["search", jql], queryFn: () => searchUseCases.search(jql), retry: false });
  const filters = useQuery({ queryKey: ["filters"], queryFn: searchUseCases.filters });

  const run = (q: string) => {
    setDraft(q);
    setParams(q ? { jql: q } : {});
  };
  const done = (text: string) => ({
    onSuccess: () => {
      setNotice({ kind: "ok" as const, text });
      return qc.invalidateQueries({ queryKey: ["filters"] });
    },
    onError: (err: Error) => setNotice({ kind: "error" as const, text: err.message }),
  });
  const save = useMutation({ mutationFn: () => searchUseCases.createFilter({ name, jql: draft, shared }), ...done(t("Filter saved")) });
  const remove = useMutation({ mutationFn: (f: Filter) => searchUseCases.deleteFilter(f.id), ...done(t("Filter deleted")) });

  function onSearch(e: FormEvent) {
    e.preventDefault();
    run(draft.trim());
  }
  function onSave(e: FormEvent) {
    e.preventDefault();
    if (name.trim()) save.mutate();
  }

  return (
    <div className="search-page">
      <h1>{t("Search")}</h1>
      <form className="row" onSubmit={onSearch}>
        <label className="grow">
          {t("JQL")}
          <input value={draft} onChange={(e) => setDraft(e.target.value)} placeholder={t("project = KYB AND status != Done ORDER BY priority DESC")} />
        </label>
        <button type="submit">{t("Search")}</button>
      </form>
      {!jql && (
        <p className="muted">
          {t("Try:")}{" "}
          {EXAMPLE_QUERIES.map((q) => (
            <button key={q} type="button" className="ghost chip" onClick={() => run(q)}>
              {q}
            </button>
          ))}
        </p>
      )}
      <div className="search-grid">
        <aside className="panel" aria-label={t("Saved filters")}>
          <h2>{t("Filters")}</h2>
          {(filters.data ?? []).length === 0 && <p className="muted">{t("No saved filters yet.")}</p>}
          <ul className="child-list">
            {filters.data?.map((f) => (
              <li key={f.id}>
                <button type="button" className="ghost" onClick={() => run(f.jql)}>
                  {f.name}
                  {f.shared && <span className="muted"> {t("· shared")}</span>}
                </button>
                {f.mine && (
                  <button type="button" className="ghost" aria-label={t("Delete filter {name}", { name: f.name })} onClick={() => remove.mutate(f)}>
                    ×
                  </button>
                )}
              </li>
            ))}
          </ul>
          <form onSubmit={onSave}>
            <label>
              {t("Save this search as")}
              <input value={name} onChange={(e) => setName(e.target.value)} maxLength={60} />
            </label>
            <label className="check">
              <input type="checkbox" checked={shared} onChange={(e) => setShared(e.target.checked)} />
              {t("Share with the workspace")}
            </label>
            <button type="submit" disabled={save.isPending || !name.trim()}>
              {t("Save filter")}
            </button>
          </form>
          {notice && (
            <p role={notice.kind === "error" ? "alert" : "status"} className={notice.kind === "error" ? "error" : "muted"}>
              {notice.text}
            </p>
          )}
        </aside>
        <section aria-label={t("Results")}>
          {results.isError && (
            <p role="alert" className="error">
              {results.error.message}
            </p>
          )}
          {results.data && (
            <>
              <p className="muted">{results.data.total === 1 ? t("1 issue") : t("{n} issues", { n: results.data.total })}</p>
              <table className="versions-table">
                <thead>
                  <tr>
                    <th scope="col">{t("Key")}</th>
                    <th scope="col">{t("Summary")}</th>
                    <th scope="col">{t("Type")}</th>
                    <th scope="col">{t("Status")}</th>
                    <th scope="col">{t("Priority")}</th>
                  </tr>
                </thead>
                <tbody>
                  {results.data.items.map((is) => (
                    <tr key={is.key}>
                      <td>
                        <Link to={`/projects/${encodeURIComponent(is.project_key)}?issue=${encodeURIComponent(is.key)}`}>{is.key}</Link>
                      </td>
                      <td>{is.title}</td>
                      <td>{is.type}</td>
                      <td>{is.status_name || is.status}</td>
                      <td>{is.priority}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </>
          )}
        </section>
      </div>
    </div>
  );
}
