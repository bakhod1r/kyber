import { type APIRequestContext, expect, test } from "@playwright/test";

// S50–S53: two people chat in a channel; messages, typing and reactions arrive live over the stream.
test("chat between two people arrives live", async ({ browser }) => {
  const id = Date.now().toString(36).slice(-5);
  const key = `C${id.toUpperCase()}`.slice(0, 6);
  const signIn = async (who: string) => {
    const ctx = await browser.newContext();
    const call = (r: APIRequestContext, method: string, path: string, data?: unknown) =>
      r.fetch(path, { method, data, headers: { "Content-Type": "application/json" } }).then((x) => (x.status() === 204 ? null : x.json()));
    const email = `${who}-${id}@example.com`;
    await call(ctx.request, "POST", "/api/v1/auth/signup", { email, name: who, password: "a very long password" });
    await call(ctx.request, "POST", "/api/v1/auth/login", { email, password: "a very long password" });
    return { ctx, email, call: (m: string, p: string, d?: unknown) => call(ctx.request, m, p, d) };
  };
  const ali = await signIn("Ali");
  const bobur = await signIn("Bobur");
  await ali.call("POST", "/api/v1/projects", { key, name: "Chat" });
  await ali.call("POST", `/api/v1/projects/${key}/members`, { email: bobur.email, role: "member" });
  await ali.call("POST", `/api/v1/projects/${key}/issues`, { title: "Login page", type: "task" });
  const channel = await ali.call("POST", "/api/v1/channels", { name: `team-${id}` });
  await bobur.call("POST", `/api/v1/conversations/${channel.id}/join`);

  const a = await ali.ctx.newPage();
  const b = await bobur.ctx.newPage();
  await a.goto(`/chat/${channel.id}`);
  await b.goto(`/chat/${channel.id}`);
  await expect(b.getByRole("heading", { name: `# team-${id}` })).toBeVisible();

  const box = a.getByRole("textbox", { name: `Message # team-${id}` });
  await box.fill(`${key}-1 is ready, @${bobur.email}`);
  await box.press("Enter");
  const log = b.getByRole("log", { name: "Messages" });
  await expect(log).toContainText(`${key}-1 is ready`); // live, without reloading
  await b.getByRole("button", { name: "React 🎉" }).click();
  await expect(a.getByRole("button", { name: "🎉 1" })).toBeVisible();

  await b.getByRole("textbox", { name: `Message # team-${id}` }).pressSequentially("hi");
  await expect(a.getByText("Bobur is typing…")).toBeVisible();

  await b.goto(`/chat/${channel.id}`); // after a reload the REST history carries the issue preview
  await expect(b.getByRole("link", { name: `${key}-1 Login page` })).toBeVisible();
  await expect.poll(async () => ((await bobur.call("GET", "/api/v1/notifications")).items as unknown[]).length).toBe(1);
});
