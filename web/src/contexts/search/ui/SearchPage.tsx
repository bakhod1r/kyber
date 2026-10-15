import { priorityLabel, typeLabel } from "../../../shared/i18n/labels";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, type KeyboardEvent, useId, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { completeJql, EXAMPLE_QUERIES, type Filter, queryProject, searchUseCases } from "../application";
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
          <JqlInput value={draft} onChange={setDraft} placeholder={t("project = KYB AND status != Done ORDER BY priority DESC")} />
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
              <table className="versions-table issue-table">
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
                      <td>{typeLabel(t, is.type)}</td>
                      <td>{is.status_name || is.status}</td>
                      <td>{priorityLabel(t, is.priority)}</td>
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

/** JqlInput: the JQL box with autocomplete (fields, operators, values, keywords) as an ARIA combobox. */
function JqlInput({ value, onChange, placeholder }: { value: string; onChange: (v: string) => void; placeholder: string }) {
  const listId = useId();
  const [focused, setFocused] = useState(false);
  const [active, setActive] = useState(-1);
  const project = queryProject(value);
  const labels = useQuery({ queryKey: ["labels", project], queryFn: () => searchUseCases.projectLabels(project), enabled: project !== "", retry: false });
  const completion = completeJql(value, { labels: labels.data ?? [], project: project ? [project] : [] });
  const open = focused && completion.options.length > 0;
  const change = (v: string) => {
    onChange(v);
    setActive(-1);
  };
  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (!open) return;
    const n = completion.options.length;
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      setActive((a) => (e.key === "ArrowDown" ? (a + 1) % n : (a - 1 + n) % n));
    } else if ((e.key === "Enter" || e.key === "Tab") && active >= 0) {
      e.preventDefault();
      change(completion.apply(completion.options[active]!));
    } else if (e.key === "Escape") {
      setFocused(false);
    }
  }
  return (
    <span className="jql-input">
      <input
        role="combobox"
        aria-expanded={open}
        aria-controls={listId}
        aria-autocomplete="list"
        aria-activedescendant={open && active >= 0 ? `${listId}-${active}` : undefined}
        value={value}
        placeholder={placeholder}
        onChange={(e) => change(e.target.value)}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        onKeyDown={onKeyDown}
      />
      {open && (
        <ul role="listbox" id={listId} className="jql-suggestions">
          {completion.options.map((o, n) => (
            <li
              key={o}
              id={`${listId}-${n}`}
              role="option"
              aria-selected={n === active}
              onMouseDown={(e) => {
                e.preventDefault();
                change(completion.apply(o));
              }}
            >
              {o}
            </li>
          ))}
        </ul>
      )}
    </span>
  );
}
