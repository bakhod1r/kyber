import { type APIRequestContext, expect, test } from "@playwright/test";

// S78: record a short video clip and post it to the channel.
test.use({ launchOptions: { executablePath: process.env.KYBER_CHROMIUM || undefined, args: ["--use-fake-ui-for-media-stream", "--use-fake-device-for-media-stream"] } });

test("record a video clip and post it", async ({ browser }) => {
  const id = Date.now().toString(36).slice(-5);
  const ctx = await browser.newContext({ permissions: ["microphone", "camera"] });
  const call = (r: APIRequestContext, method: string, path: string, data?: unknown) =>
    r.fetch(path, { method, data: data ?? {}, headers: { "Content-Type": "application/json" } }).then((x) => x.json());
  const email = `clip-${id}@example.com`;
  await call(ctx.request, "POST", "/api/v1/auth/signup", { email, name: "Ali", password: "a very long password" });
  await call(ctx.request, "POST", "/api/v1/auth/login", { email, password: "a very long password" });
  const channel = await call(ctx.request, "POST", "/api/v1/channels", { name: `clips-${id}` });

  const a = await ctx.newPage();
  await a.goto(`/chat/${channel.id}`);
  await a.getByLabel("Record a clip").selectOption("video");
  await expect(a.getByRole("group", { name: "Recording a clip" })).toContainText("/ 5:00");
  await a.waitForTimeout(2000);
  await a.getByRole("button", { name: "Stop and send" }).click();
  const clip = a.getByRole("article").filter({ hasText: "Video clip" });
  await expect(clip.locator("video")).toBeVisible({ timeout: 15_000 });
});
