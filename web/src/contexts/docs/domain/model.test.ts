import { describe, expect, it } from "vitest";
import { parseInline, parseMarkdown, safeHref } from "./model";

describe("markdown", () => {
  it("reads inline marks and keeps unsafe links as text", () => {
    expect(parseInline("a **b** `d` [e](https://x.uz) [f](javascript:alert(1))")).toEqual([
      { kind: "text", text: "a " },
      { kind: "strong", children: [{ kind: "text", text: "b" }] },
      { kind: "text", text: " " },
      { kind: "code", text: "d" },
      { kind: "text", text: " " },
      { kind: "link", href: "https://x.uz", children: [{ kind: "text", text: "e" }] },
      { kind: "text", text: " " },
      { kind: "text", text: "[f](javascript:alert(1)" },
      { kind: "text", text: ")" },
    ]);
    expect(parseInline("*em*")).toEqual([{ kind: "em", children: [{ kind: "text", text: "em" }] }]);
    expect(safeHref("mailto:a@x.uz")).toBe(true);
    expect(safeHref("data:text/html,x")).toBe(false);
  });

  it("reads blocks", () => {
    const md = "# Title\n\nFirst line\nsecond line\n\n- one\n- two\n1. a\n2) b\n> quoted\n> more\n\n```\n<b>raw</b>\n```\n---\n###### Small\npara\n```\nunclosed";
    expect(parseMarkdown(md)).toEqual([
      { kind: "heading", level: 1, inline: [{ kind: "text", text: "Title" }] },
      { kind: "paragraph", inline: [{ kind: "text", text: "First line second line" }] },
      { kind: "list", ordered: false, items: [[{ kind: "text", text: "one" }], [{ kind: "text", text: "two" }]] },
      { kind: "list", ordered: true, items: [[{ kind: "text", text: "a" }], [{ kind: "text", text: "b" }]] },
      { kind: "quote", inline: [{ kind: "text", text: "quoted more" }] },
      { kind: "code", text: "<b>raw</b>" },
      { kind: "rule" },
      { kind: "heading", level: 6, inline: [{ kind: "text", text: "Small" }] },
      { kind: "paragraph", inline: [{ kind: "text", text: "para" }] },
      { kind: "code", text: "unclosed" },
    ]);
    expect(parseMarkdown("text\n- item")).toEqual([
      { kind: "paragraph", inline: [{ kind: "text", text: "text" }] },
      { kind: "list", ordered: false, items: [[{ kind: "text", text: "item" }]] },
    ]);
  });
});
