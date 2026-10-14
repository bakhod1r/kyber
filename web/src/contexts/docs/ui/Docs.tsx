import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { docsUseCases, type Page } from "../application";
import { Markdown } from "../../../shared/ui/Markdown";
import { useT } from "../../../shared/i18n/i18n";

type Mode = { kind: "view" } | { kind: "edit"; page: Page | null } | { kind: "history" };

/** Docs: the project's Markdown pages with their revision history. */
export function Docs({ projectKey }: { projectKey: string }) {
  const t = useT();
  const qc = useQueryClient();
  const listKey = ["docs", projectKey];
  const pages = useQuery({ queryKey: listKey, queryFn: () => docsUseCases.pages(projectKey) });
  const [selected, setSelected] = useState<string | null>(null);
  const [mode, setMode] = useState<Mode>({ kind: "view" });
  const [error, setError] = useState("");
  const current = pages.data?.find((p) => p.id === selected) ?? pages.data?.[0] ?? null;
  const done = (p: Page | null) => {
    setError("");
    setMode({ kind: "view" });
    setSelected(p?.id ?? null);
    return qc.invalidateQueries({ queryKey: listKey });
  };
  const remove = useMutation({ mutationFn: (p: Page) => docsUseCases.deletePage(p.id), onSuccess: () => done(null), onError: (e: Error) => setError(e.message) });

  return (
    <section className="docs" aria-label={t("Docs")}>
      <h2>{t("Docs")}</h2>
      <div className="docs-grid">
        <nav aria-label={t("Pages")}>
          <button type="button" onClick={() => setMode({ kind: "edit", page: null })}>
            {t("New page")}
          </button>
          {pages.isError && (
            <p role="alert" className="error">
              {(pages.error as Error).message}
            </p>
          )}
          {pages.data?.length === 0 && <p className="muted">{t("No pages yet.")}</p>}
          <ul>
            {pages.data?.map((p) => (
              <li key={p.id}>
                <button
                  type="button"
                  className="ghost"
                  aria-current={p.id === current?.id && mode.kind !== "edit" ? "page" : undefined}
                  onClick={() => {
                    setSelected(p.id);
                    setMode({ kind: "view" });
                  }}
                >
                  {p.title}
                </button>
              </li>
            ))}
          </ul>
        </nav>
        <div className="docs-page">
          {error && (
            <p role="alert" className="error">
              {error}
            </p>
          )}
          {mode.kind === "edit" ? (
            <PageEditor key={mode.page?.id ?? "new"} projectKey={projectKey} page={mode.page} onSaved={done} onCancel={() => setMode({ kind: "view" })} />
          ) : current ? (
            <article aria-label={current.title}>
              <div className="row between">
                <h3>{current.title}</h3>
                <div className="row">
                  <button type="button" onClick={() => setMode({ kind: "edit", page: current })}>
                    {t("Edit")}
                  </button>
                  <button type="button" className="ghost" onClick={() => setMode(mode.kind === "history" ? { kind: "view" } : { kind: "history" })}>
                    {mode.kind === "history" ? t("Hide history") : t("History")}
                  </button>
                  <button type="button" className="ghost" onClick={() => remove.mutate(current)}>
                    {t("Delete page")}
                  </button>
                </div>
              </div>
              <p className="muted">
                {t("Version {n} · updated {at}", { n: current.version, at: current.updated_at.slice(0, 16).replace("T", " ") })}
              </p>
              {mode.kind === "history" ? <PageHistory id={current.id} /> : <Markdown source={current.body} />}
            </article>
          ) : null}
        </div>
      </div>
    </section>
  );
}

function PageEditor({ projectKey, page, onSaved, onCancel }: { projectKey: string; page: Page | null; onSaved: (p: Page) => void; onCancel: () => void }) {
  const t = useT();
  const [title, setTitle] = useState(page?.title ?? "");
  const [body, setBody] = useState(page?.body ?? "");
  const [preview, setPreview] = useState(false);
  const save = useMutation({
    mutationFn: () => (page ? docsUseCases.revisePage(page.id, page.version, { title, body }) : docsUseCases.createPage(projectKey, { title, body })),
    onSuccess: onSaved,
  });
  const submit = (e: FormEvent) => {
    e.preventDefault();
    save.mutate();
  };
  return (
    <form className="page-editor" onSubmit={submit}>
      <label>
        {t("Page title")}
        <input value={title} onChange={(e) => setTitle(e.target.value)} required maxLength={200} />
      </label>
      {preview ? (
        <Markdown source={body} />
      ) : (
        <label>
          {t("Content (Markdown)")}
          <textarea rows={16} value={body} onChange={(e) => setBody(e.target.value)} />
        </label>
      )}
      {save.isError && (
        <p role="alert" className="error">
          {save.error.message}
        </p>
      )}
      <div className="row">
        <button type="submit" disabled={save.isPending}>
          {t("Save page")}
        </button>
        <button type="button" className="ghost" onClick={() => setPreview(!preview)}>
          {preview ? t("Write") : t("Preview")}
        </button>
        <button type="button" className="ghost" onClick={onCancel}>
          {t("Cancel")}
        </button>
      </div>
    </form>
  );
}

function PageHistory({ id }: { id: string }) {
  const t = useT();
  const history = useQuery({ queryKey: ["docs-history", id], queryFn: () => docsUseCases.history(id) });
  const [open, setOpen] = useState<number | null>(null);
  const shown = history.data?.find((r) => r.number === open);
  return (
    <section aria-label={t("Page history")}>
      <ol className="page-history">
        {history.data?.map((r) => (
          <li key={r.number}>
            <button type="button" className="ghost" onClick={() => setOpen(r.number)}>
              {t("Version {n}", { n: r.number })}
            </button>{" "}
            <span className="muted">
              {r.at.slice(0, 16).replace("T", " ")} · {r.title}
            </span>
          </li>
        ))}
      </ol>
      {shown && <Markdown source={shown.body} />}
    </section>
  );
}
