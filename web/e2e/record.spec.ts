import { type APIRequestContext, expect, test } from "@playwright/test";

// S62: record a huddle; everyone sees the banner, and the video lands in the conversation when it stops.
test.use({ launchOptions: { executablePath: process.env.KYBER_CHROMIUM || undefined, args: ["--use-fake-ui-for-media-stream", "--use-fake-device-for-media-stream"] } });

test("record a huddle and post the video to the channel", async ({ browser }) => {
  const id = Date.now().toString(36).slice(-5);
  const signIn = async (who: string) => {
    const ctx = await browser.newContext({ permissions: ["microphone", "camera"] });
    const call = (r: APIRequestContext, method: string, path: string, data?: unknown) =>
      r.fetch(path, { method, data, headers: { "Content-Type": "application/json" } }).then((x) => (x.status() === 204 ? null : x.json()));
    const email = `${who}-${id}@example.com`;
    await call(ctx.request, "POST", "/api/v1/auth/signup", { email, name: who, password: "a very long password" });
    await call(ctx.request, "POST", "/api/v1/auth/login", { email, password: "a very long password" });
    return { ctx, email, call: (m: string, p: string, d?: unknown) => call(ctx.request, m, p, d) };
  };
  const ali = await signIn("Ali");
  const bobur = await signIn("Bobur");
  const key = `R${id.toUpperCase()}`.slice(0, 6);
  await ali.call("POST", "/api/v1/projects", { key, name: "Record" }); // people see each other's names through a project
  await ali.call("POST", `/api/v1/projects/${key}/members`, { email: bobur.email, role: "member" });
  const channel = await ali.call("POST", "/api/v1/channels", { name: `rec-${id}` });
  await bobur.call("POST", `/api/v1/conversations/${channel.id}/join`);

  const a = await ali.ctx.newPage();
  const b = await bobur.ctx.newPage();
  await a.goto(`/chat/${channel.id}`);
  await b.goto(`/chat/${channel.id}`);
  await a.getByRole("button", { name: "Start huddle" }).click();
  await b.getByRole("button", { name: "Join huddle (1)" }).click();
  await a.getByRole("button", { name: "Start camera" }).click();

  await a.getByRole("button", { name: "Record" }).click();
  await expect(b.getByText("● Recording by Ali")).toBeVisible();
  await expect(b.getByRole("button", { name: "Record" })).toHaveCount(0); // one recorder at a time
  await a.waitForTimeout(2500);
  await a.getByRole("button", { name: "Stop recording" }).click();
  await expect(b.getByText("● Recording by Ali")).toHaveCount(0);

  const posted = b.getByRole("article").filter({ hasText: "recorded the huddle" });
  await expect(posted).toBeVisible({ timeout: 20_000 });
  await expect(posted.locator("video")).toBeVisible();
});
