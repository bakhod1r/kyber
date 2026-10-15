// The stylesheet's colour contract: every theme defines the same tokens, nothing reads an undefined
// token, and component colours come from tokens so they follow the chosen theme.
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const css = readFileSync(join(__dirname, "..", "..", "styles.css"), "utf8");

/** Tokens every theme must set, so a component never inherits another theme's value. */
const THEME_TOKENS = [
  "--bg", "--surface", "--surface-2", "--border", "--text", "--muted", "--accent", "--accent-text",
  "--danger", "--success", "--warning", "--glow", "--shadow",
  "--viz-1", "--viz-2", "--viz-muted", "--viz-grid",
];
/** Set at runtime by src/shared/appearance.ts. */
const RUNTIME_TOKENS = new Set(["--wallpaper-custom"]);

function block(selector: string): string {
  const at = css.indexOf(`${selector} {`);
  expect(at, selector).toBeGreaterThanOrEqual(0);
  return css.slice(at, css.indexOf("}", at));
}

describe("styles.css theme tokens", () => {
  const themes = [...css.matchAll(/:root\[data-theme="(\w+)"\] \{/g)].map((m) => m[1] ?? "");

  it("has the themes the appearance settings offer", () => {
    expect(themes).toEqual(expect.arrayContaining(["light", "dark", "midnight", "ocean", "forest", "sunset", "rose", "nord", "dracula", "contrast"]));
  });

  it.each([":root", ...themes.map((t) => `:root[data-theme="${t}"]`)])("%s sets every theme token", (sel) => {
    const body = block(sel);
    for (const tok of THEME_TOKENS) expect(body, `${sel} ${tok}`).toContain(`${tok}:`);
  });

  it("reads no undefined custom property", () => {
    const defined = new Set([...css.matchAll(/(--[\w-]+)\s*:/g)].map((m) => m[1] ?? ""));
    const used = new Set([...css.matchAll(/var\((--[\w-]+)/g)].map((m) => m[1] ?? ""));
    const missing = [...used].filter((v) => !defined.has(v) && !RUNTIME_TOKENS.has(v));
    expect(missing).toEqual([]);
  });

  it("does not hide a missing token behind a hard-coded fallback", () => {
    expect(css.match(/var\(--[\w-]+,\s*#[0-9a-f]+\)/gi) ?? []).toEqual([]);
  });

  it("chart colours follow the chosen theme, not only the OS setting", () => {
    expect(css).not.toMatch(/\.viz-root\s*\{[^}]*--viz-grid:\s*#/);
  });

  it("the system theme follows the OS only when no theme is chosen", () => {
    expect(css).toContain(':root:not([data-theme]) {');
  });
});
