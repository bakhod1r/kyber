// docs: domain model — project pages and their history (Markdown is read by shared/markdown).
export type Page = {
  id: string;
  project_key: string;
  title: string;
  body: string;
  author_id: string;
  updated_by: string;
  updated_at: string;
  version: number;
};
export type PageInput = { title: string; body: string };
export type PageRevision = { number: number; title: string; body: string; by: string; at: string };

/** S86: a remark on one paragraph of a page; `quote` is the start of the paragraph's text when it was made. */
export type DocComment = { id: string; page_id: string; paragraph: number; quote: string; author_id: string; body: string; created_at: string };

/** paragraphs splits a page into its blocks (separated by blank lines); a fenced code block stays one block. */
export function paragraphs(body: string): string[] {
  const out: string[] = [];
  let cur: string[] = [];
  let fence = false;
  const flush = () => {
    if (cur.join("").trim()) out.push(cur.join("\n"));
    cur = [];
  };
  for (const line of body.split("\n")) {
    if (/^\s*(```|~~~)/.test(line)) fence = !fence;
    if (!fence && line.trim() === "") flush();
    else cur.push(line);
  }
  flush();
  return out;
}

/** quoteOf: what a comment remembers of its paragraph. */
export const quoteOf = (paragraph: string) => paragraph.trim().slice(0, 80);

/** placeComment: the paragraph a comment belongs to now — its own index while the text still starts with its quote,
 * else the first paragraph that does; -1 when the paragraph is gone (the comment is on an earlier version). */
export function placeComment(paras: string[], c: Pick<DocComment, "paragraph" | "quote">): number {
  const fits = (p: string | undefined) => p !== undefined && p.trim().startsWith(c.quote);
  if (fits(paras[c.paragraph])) return c.paragraph;
  return paras.findIndex(fits);
}
