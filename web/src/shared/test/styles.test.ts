// The stylesheet's colour contract: every theme defines the same tokens, nothing reads an undefined
// token, and component colours come from tokens so they follow the chosen theme.
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";

const SRC = join(__dirname, "..", "..");
const entry = readFileSync(join(SRC, "styles.css"), "utf8");
/** The stylesheet split into ordered parts under src/styles, imported by src/styles.css. */
const parts = [...entry.matchAll(/@import "\.\/styles\/([\w-]+\.css)";/g)].map((m) => m[1] ?? "");
const css = parts.map((f) => readFileSync(join(SRC, "styles", f), "utf8")).join("\n");

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

describe("styles.css layout", () => {
  it("imports every part under src/styles, in a fixed order", () => {
    expect(parts).toEqual(["base.css", "components.css", "themes.css", "responsive.css", "motion.css"]);
    expect(readdirSync(join(SRC, "styles")).sort()).toEqual([...parts].sort());
  });

  it("turns every animation off for people who ask for reduced motion", () => {
    const motion = readFileSync(join(SRC, "styles", "motion.css"), "utf8");
    expect(motion).toMatch(/@media \(prefers-reduced-motion: reduce\) \{\s*\*, \*::before, \*::after \{[^}]*animation: none !important;[^}]*transition: none !important;/);
  });

  it("stacks layers through z-index tokens, not ad hoc numbers", () => {
    expect(css.match(/z-index:\s*\d{2,}/g) ?? []).toEqual([]);
  });
});

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
