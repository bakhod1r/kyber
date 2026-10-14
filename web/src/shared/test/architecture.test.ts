// Classic DDD in the frontend (ADR-0005): each bounded context under src/contexts/<context>/ has
// domain (types, pure rules) → infrastructure (HTTP gateway) → application (use cases) → ui.
// The app shell (src/app) composes contexts; shared code never depends on a context.
import { readFileSync, readdirSync, statSync } from "node:fs";
import { dirname, join, normalize, relative } from "node:path";
import { describe, expect, it } from "vitest";

const SRC = join(__dirname, "..", "..");

function files(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) return files(p);
    return /\.tsx?$/.test(name) && !/\.test\.tsx?$/.test(name) ? [p] : [];
  });
}

type Where = { area: string; context?: string; layer?: string };

function where(file: string): Where {
  const parts = relative(SRC, file).split("/");
  if (parts[0] === "contexts") return { area: "contexts", context: parts[1], layer: parts[2] };
  return { area: parts[0] ?? "" };
}

function violations(): string[] {
  const out: string[] = [];
  for (const file of files(SRC)) {
    const from = where(file);
    const text = readFileSync(file, "utf8");
    for (const m of text.matchAll(/(?:from|import)\s*\(?\s*"([^"]+)"/g)) {
      const spec = m[1] ?? "";
      const name = relative(SRC, file);
      if (!spec.startsWith(".")) {
        if (from.layer === "domain") out.push(`${name}: the domain imports no packages (${spec})`);
        continue;
      }
      const to = where(normalize(join(dirname(file), spec)));
      const bad = (why: string) => out.push(`${name} → ${spec}: ${why}`);
      if (from.area === "shared" && to.area === "contexts") bad("shared code never depends on a context");
      if (from.area !== "contexts") continue;
      const own = to.context === from.context;
      switch (from.layer) {
        case "domain":
          if (to.layer !== "domain") bad("the domain depends only on domain models");
          break;
        case "infrastructure":
          if (to.area === "contexts" && to.layer !== "domain") bad("infrastructure depends on domain models and shared/http");
          if (to.area === "app") bad("infrastructure never depends on the app shell");
          break;
        case "application":
          if (to.area === "contexts" && !(to.layer === "domain" || (own && (to.layer === "infrastructure" || to.layer === "application"))))
            bad("use cases depend on domain models, their own gateway and their own use cases");
          break;
        case "ui":
          if (to.layer === "infrastructure") bad("the UI goes through the application layer, never infrastructure");
          if (to.area === "contexts" && !own && to.layer === "ui") bad("contexts' UIs are composed by the app shell, not by each other");
          if (to.area === "app") bad("a context never depends on the app shell");
          break;
        default:
          bad(`unknown layer ${from.layer}`);
      }
    }
  }
  return out;
}

describe("frontend architecture", () => {
  it("keeps the four DDD layers and context boundaries", () => {
    expect(files(SRC).length).toBeGreaterThan(30);
    expect(violations()).toEqual([]);
  });
});
