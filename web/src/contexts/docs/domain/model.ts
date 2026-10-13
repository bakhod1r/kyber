// docs: domain model — project pages, their history and a small, safe Markdown reader.
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

/** Inline Markdown: plain text, **strong**, *emphasis*, `code` and [links](https://…). */
export type Inline =
  | { kind: "text"; text: string }
  | { kind: "strong" | "em"; children: Inline[] }
  | { kind: "code"; text: string }
  | { kind: "link"; href: string; children: Inline[] };

/** Block Markdown: headings, paragraphs, lists, quotes, fenced code and rules. No raw HTML. */
export type Block =
  | { kind: "heading"; level: 1 | 2 | 3 | 4 | 5 | 6; inline: Inline[] }
  | { kind: "paragraph"; inline: Inline[] }
  | { kind: "list"; ordered: boolean; items: Inline[][] }
  | { kind: "quote"; inline: Inline[] }
  | { kind: "code"; text: string }
  | { kind: "rule" };

/** Only web and mail links are kept; anything else (javascript:, data:) stays text. */
export const safeHref = (href: string) => /^(https?:\/\/|mailto:|\/|#)/i.test(href.trim());

const INLINE = /(`[^`]+`)|(\*\*[^*]+\*\*)|(\*[^*]+\*)|(\[[^\]]+\]\([^)\s]+\))/;

export function parseInline(src: string): Inline[] {
  const out: Inline[] = [];
  let rest = src;
  while (rest) {
    const m = INLINE.exec(rest);
    if (!m) {
      out.push({ kind: "text", text: rest });
      break;
    }
    if (m.index > 0) out.push({ kind: "text", text: rest.slice(0, m.index) });
    const tok = m[0];
    if (m[1]) out.push({ kind: "code", text: tok.slice(1, -1) });
    else if (m[2]) out.push({ kind: "strong", children: parseInline(tok.slice(2, -2)) });
    else if (m[3]) out.push({ kind: "em", children: parseInline(tok.slice(1, -1)) });
    else {
      const close = tok.indexOf("](");
      const label = tok.slice(1, close);
      const href = tok.slice(close + 2, -1);
      out.push(safeHref(href) ? { kind: "link", href, children: parseInline(label) } : { kind: "text", text: tok });
    }
    rest = rest.slice(m.index + tok.length);
  }
  return out;
}

const BULLET = /^\s*[-*+]\s+(.*)$/;
const NUMBERED = /^\s*\d+[.)]\s+(.*)$/;

/** parseMarkdown reads a page body into blocks for rendering as elements (never as HTML). */
export function parseMarkdown(src: string): Block[] {
  const lines = src.replace(/\r\n?/g, "\n").split("\n");
  const blocks: Block[] = [];
  const at = (k: number) => lines[k] ?? "";
  let i = 0;
  while (i < lines.length) {
    const line = at(i);
    if (line.trim() === "") {
      i++;
      continue;
    }
    if (line.startsWith("```")) {
      const code: string[] = [];
      for (i++; i < lines.length && !at(i).startsWith("```"); i++) code.push(at(i));
      i++; // closing fence (or end)
      blocks.push({ kind: "code", text: code.join("\n") });
      continue;
    }
    const h = /^(#{1,6})\s+(.*)$/.exec(line);
    if (h) {
      blocks.push({ kind: "heading", level: (h[1] ?? "").length as 1 | 2 | 3 | 4 | 5 | 6, inline: parseInline((h[2] ?? "").trim()) });
      i++;
      continue;
    }
    if (/^\s*([-*_])(\s*\1){2,}\s*$/.test(line)) {
      blocks.push({ kind: "rule" });
      i++;
      continue;
    }
    const listRe = BULLET.test(line) ? BULLET : NUMBERED.test(line) ? NUMBERED : null;
    if (listRe) {
      const items: Inline[][] = [];
      for (let m = listRe.exec(at(i)); i < lines.length && m; m = listRe.exec(at(++i))) items.push(parseInline(m[1] ?? ""));
      blocks.push({ kind: "list", ordered: listRe === NUMBERED, items });
      continue;
    }
    const para: string[] = [];
    const quote = line.startsWith(">");
    while (i < lines.length && at(i).trim() !== "" && at(i).startsWith(">") === quote && !/^(#{1,6}\s|```)/.test(at(i)) && !(!quote && (BULLET.test(at(i)) || NUMBERED.test(at(i))))) {
      para.push(quote ? at(i).replace(/^>\s?/, "") : at(i).trim());
      i++;
    }
    blocks.push({ kind: quote ? "quote" : "paragraph", inline: parseInline(para.join(" ")) });
  }
  return blocks;
}
