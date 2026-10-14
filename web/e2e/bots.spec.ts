import { type APIRequestContext, expect, test } from "@playwright/test";

// S84: a (site) admin adds a bot, invites it to a channel; the bot posts with its token.
test("a bot posts to a channel with its token", async ({ browser }) => {
  const id = Date.now().toString(36).slice(-5);
  const ctx = await browser.newContext();
  const call = (r: APIRequestContext, method: string, path: string, data?: unknown) =>
    r.fetch(path, { method, data: data ?? {}, headers: { "Content-Type": "application/json" } }).then((x) => (x.status() === 204 ? null : x.json()));
  // The e2e server makes admin@e2e.test a site admin (Makefile); sign up once, or log in.
  await call(ctx.request, "POST", "/api/v1/auth/signup", { email: "admin@e2e.test", name: "Admin", password: "a very long password" });
  await call(ctx.request, "POST", "/api/v1/auth/login", { email: "admin@e2e.test", password: "a very long password" });
  const channel = await call(ctx.request, "POST", "/api/v1/channels", { name: `builds-${id}` });

  const a = await ctx.newPage();
  await a.goto("/chat");
  await a.getByLabel("Bot name").fill(`CI ${id}`);
  await a.getByRole("button", { name: "Add bot" }).click();
  const token = await a.getByLabel("Bot token value").inputValue();
  expect(token).toMatch(/^kpat_/);
  await a.getByRole("button", { name: "Done" }).click();

  await a.goto(`/chat/${channel.id}`);
  await a.getByRole("button", { name: "⚙ Settings" }).click();
  await a.getByLabel("Invite to channel").selectOption({ label: `CI ${id}` });
  await expect(a.getByRole("list", { name: "Channel members" })).toContainText(`CI ${id}`);

  const res = await fetch(`${process.env.KYBER_E2E_URL ?? "http://localhost:18080"}/api/v1/conversations/${channel.id}/messages`, {
    method: "POST", headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" }, body: JSON.stringify({ body: "build #7 passed" }),
  });
  expect(res.status).toBe(201);
  const msg = a.getByRole("article").filter({ hasText: "build #7 passed" });
  await expect(msg).toContainText(`CI ${id}`);
  await expect(msg.getByTitle("Bot")).toHaveText(/🤖/);
});
