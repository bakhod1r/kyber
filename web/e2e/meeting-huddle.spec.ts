import { type APIRequestContext, expect, test } from "@playwright/test";

// S76: a meeting held in a channel's huddle; "Join huddle" from the time table joins it right away.
test.use({ launchOptions: { executablePath: process.env.KYBER_CHROMIUM || undefined, args: ["--use-fake-ui-for-media-stream", "--use-fake-device-for-media-stream"] } });

test("join a meeting's huddle from the time table", async ({ browser }) => {
  const id = Date.now().toString(36).slice(-5);
  const key = `M${id.toUpperCase()}`.slice(0, 6);
  const signIn = async (who: string) => {
    const ctx = await browser.newContext({ permissions: ["microphone", "camera"] });
    const call = (r: APIRequestContext, method: string, path: string, data?: unknown) =>
      r.fetch(path, { method, data: data ?? {}, headers: { "Content-Type": "application/json" } }).then((x) => (x.status() === 204 ? null : x.json()));
    const email = `${who}-${id}@example.com`;
    await call(ctx.request, "POST", "/api/v1/auth/signup", { email, name: who, password: "a very long password" });
    await call(ctx.request, "POST", "/api/v1/auth/login", { email, password: "a very long password" });
    return { ctx, email, call: (m: string, p: string, d?: unknown) => call(ctx.request, m, p, d) };
  };
  const ali = await signIn("Ali");
  const bobur = await signIn("Bobur");
  await ali.call("POST", "/api/v1/projects", { key, name: "Meet" });
  await ali.call("POST", `/api/v1/projects/${key}/members`, { email: bobur.email, role: "member" });
  const channel = await ali.call("POST", "/api/v1/channels", { name: `sync-${id}` });
  await bobur.call("POST", `/api/v1/conversations/${channel.id}/join`);

  const a = await ali.ctx.newPage();
  await a.goto("/meetings");
  await a.getByLabel("Title").fill("Weekly sync");
  const start = new Date(Date.now() + 60 * 60_000);
  const local = new Date(start.getTime() - start.getTimezoneOffset() * 60_000).toISOString().slice(0, 16);
  await a.getByLabel("Starts").fill(local);
  await a.getByLabel("Bobur").check();
  await a.getByLabel("Huddle in").selectOption({ label: `#sync-${id}` });
  await a.getByRole("button", { name: "Schedule" }).click();
  await expect(a.getByRole("list", { name: "Meetings" })).toContainText("Weekly sync");

  const b = await bobur.ctx.newPage();
  await b.goto("/meetings");
  await b.getByRole("link", { name: "Join the huddle of Weekly sync" }).click();
  await expect(b.getByRole("list", { name: "Participants" })).toContainText("You");
  await expect(b).toHaveURL(new RegExp(`/chat/${channel.id}`));
});
