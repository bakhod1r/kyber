import { afterEach, describe, expect, it, vi } from "vitest";
import { demoFetch } from "./install";
import { DemoServer } from "./demoServer";

afterEach(() => vi.restoreAllMocks());

describe("demoFetch", () => {
  it("answers /api from the demo server and passes everything else through", async () => {
    const passthrough = vi.fn(async () => new Response("asset"));
    const f = demoFetch(new DemoServer(), passthrough);
    const me = await f("/api/v1/me", { method: "GET" });
    expect(await me.json()).toMatchObject({ name: "Demo User" });
    const created = await f("/api/v1/projects", { method: "POST", body: JSON.stringify({ key: "OPS", name: "Ops" }) });
    expect(created.status).toBe(201);
    expect(await (await f("/logo.svg")).text()).toBe("asset");
    expect(passthrough).toHaveBeenCalledTimes(1);
  });

  it("returns a 204 without a body", async () => {
    const res = await demoFetch(new DemoServer(), fetch)("/api/v1/auth/logout", { method: "POST" });
    expect([res.status, await res.text()]).toEqual([204, ""]);
  });
});
