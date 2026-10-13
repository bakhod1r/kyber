import type { ReactNode } from "react";
import { type Inline, parseMarkdown } from "../markdown";

function inline(nodes: Inline[]): ReactNode[] {
  return nodes.map((n, i) => {
    switch (n.kind) {
      case "text":
        return n.text;
      case "code":
        return <code key={i}>{n.text}</code>;
      case "strong":
        return <strong key={i}>{inline(n.children)}</strong>;
      case "em":
        return <em key={i}>{inline(n.children)}</em>;
      case "link":
        return (
          <a key={i} href={n.href} rel="noopener noreferrer" target="_blank">
            {inline(n.children)}
          </a>
        );
    }
  });
}

/** Markdown renders a page body as React elements — never as raw HTML, so pages cannot inject markup. */
export function Markdown({ source }: { source: string }) {
  return (
    <div className="markdown">
      {parseMarkdown(source).map((b, i) => {
        switch (b.kind) {
          case "heading": {
            const H = `h${Math.min(b.level + 2, 6)}` as "h3";
            return <H key={i}>{inline(b.inline)}</H>;
          }
          case "paragraph":
            return <p key={i}>{inline(b.inline)}</p>;
          case "quote":
            return <blockquote key={i}>{inline(b.inline)}</blockquote>;
          case "code":
            return (
              <pre key={i}>
                <code>{b.text}</code>
              </pre>
            );
          case "rule":
            return <hr key={i} />;
          case "list": {
            const L = b.ordered ? "ol" : "ul";
            return (
              <L key={i}>
                {b.items.map((it, j) => (
                  <li key={j}>{inline(it)}</li>
                ))}
              </L>
            );
          }
        }
      })}
    </div>
  );
}
