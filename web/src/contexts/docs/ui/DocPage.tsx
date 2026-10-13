import { useQuery } from "@tanstack/react-query";
import { docsUseCases } from "../application";
import { Markdown } from "./Markdown";

/** A read-only doc page (what a huddle shows when the page is presented). Readers without access see a note. */
export function DocPage({ id }: { id: string }) {
  const page = useQuery({ queryKey: ["doc", id], queryFn: () => docsUseCases.page(id) });
  if (page.isLoading) return <p className="muted">Loading…</p>;
  if (page.error) return <p className="muted">You cannot open this page.</p>;
  return (
    <article className="doc-page" aria-label={`Doc ${page.data?.title ?? ""}`}>
      <h2>{page.data?.title}</h2>
      <Markdown source={page.data?.body ?? ""} />
    </article>
  );
}
