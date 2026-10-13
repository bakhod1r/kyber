import { type APIRequestContext, expect, test } from "@playwright/test";

// S64: Activity lists my mentions and unread conversations; opening the conversation reads them.
test("activity shows mentions and unread conversations", async ({ browser }) => {
  const id = Date.now().toString(36).slice(-5);
  const key = `A${id.toUpperCase()}`.slice(0, 6);
  const signIn = async (who: string) => {
    const ctx = await browser.newContext();
    const call = (r: APIRequestContext, method: string, path: string, data?: unknown) =>
      r.fetch(path, { method, data: data ?? {}, headers: { "Content-Type": "application/json" } }).then((x) => (x.status() === 204 ? null : x.json()));
    const email = `${who}-${id}@example.com`;
    await call(ctx.request, "POST", "/api/v1/auth/signup", { email, name: who, password: "a very long password" });
    await call(ctx.request, "POST", "/api/v1/auth/login", { email, password: "a very long password" });
    return { ctx, email, call: (m: string, p: string, d?: unknown) => call(ctx.request, m, p, d) };
  };
  const ali = await signIn("Ali");
  const bobur = await signIn("Bobur");
  await ali.call("POST", "/api/v1/projects", { key, name: "Activity" });
  await ali.call("POST", `/api/v1/projects/${key}/members`, { email: bobur.email, role: "member" });
  const channel = await ali.call("POST", "/api/v1/channels", { name: `act-${id}` });
  await bobur.call("POST", `/api/v1/conversations/${channel.id}/join`);
  await ali.call("POST", `/api/v1/conversations/${channel.id}/messages`, { body: `@${bobur.email} can you check the deploy?` });

  await expect.poll(async () => (await bobur.call("GET", "/api/v1/notifications")).unread).toBe(1); // delivered by the relay
  const b = await bobur.ctx.newPage();
  await b.goto("/chat");
  const mentions = b.getByRole("list", { name: "Mentions" });
  await expect(mentions).toContainText("can you check the deploy?");
  await expect(b.getByRole("list", { name: "Unread conversations" })).toContainText(`# act-${id}`);

  await mentions.getByRole("link", { name: `Ali in #act-${id}` }).click();
  await expect(b.getByRole("log", { name: "Messages" })).toContainText("can you check the deploy?");
  await b.getByRole("link", { name: "🔔 Activity" }).click();
  await expect(b.getByText("You are all caught up.")).toBeVisible();
  await expect.poll(async () => (await bobur.call("GET", "/api/v1/notifications")).unread).toBe(0);
});
