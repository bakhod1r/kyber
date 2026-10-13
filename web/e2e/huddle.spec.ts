import { type APIRequestContext, type Page, expect, test } from "@playwright/test";

// S57: a huddle between two browsers — real WebRTC (fake camera/microphone devices), camera and screen share.
test.use({
  launchOptions: {
    executablePath: process.env.KYBER_CHROMIUM || undefined,
    args: ["--use-fake-ui-for-media-stream", "--use-fake-device-for-media-stream", "--auto-accept-this-tab-capture", "--auto-select-desktop-capture-source=Entire screen"],
  },
});

/** Waits until a <video> really renders frames from the peer. */
async function playing(page: Page, label: string) {
  await expect
    .poll(() => page.getByLabel(label).evaluate((v: HTMLVideoElement) => v.videoWidth * (v.readyState >= 2 ? 1 : 0)), { timeout: 15_000 })
    .toBeGreaterThan(0);
}

test("two people meet in a huddle with camera and screen share", async ({ browser }) => {
  const id = Date.now().toString(36).slice(-5);
  const key = `H${id.toUpperCase()}`.slice(0, 6);
  const signIn = async (who: string) => {
    const ctx = await browser.newContext({ permissions: ["camera", "microphone"] });
    const call = (r: APIRequestContext, method: string, path: string, data?: unknown) =>
      r.fetch(path, { method, data, headers: { "Content-Type": "application/json" } }).then((x) => (x.status() === 204 ? null : x.json()));
    const email = `${who}-${id}@example.com`;
    await call(ctx.request, "POST", "/api/v1/auth/signup", { email, name: who, password: "a very long password" });
    await call(ctx.request, "POST", "/api/v1/auth/login", { email, password: "a very long password" });
    return { ctx, email, call: (m: string, p: string, d?: unknown) => call(ctx.request, m, p, d) };
  };
  const ali = await signIn("Ali");
  const bobur = await signIn("Bobur");
  await ali.call("POST", "/api/v1/projects", { key, name: "Huddle" });
  await ali.call("POST", `/api/v1/projects/${key}/members`, { email: bobur.email, role: "member" });
  const channel = await ali.call("POST", "/api/v1/channels", { name: `meet-${id}` });
  await bobur.call("POST", `/api/v1/conversations/${channel.id}/join`);

  const a = await ali.ctx.newPage();
  const b = await bobur.ctx.newPage();
  await a.goto(`/chat/${channel.id}`);
  await b.goto(`/chat/${channel.id}`);
  await a.getByRole("button", { name: "Start huddle" }).click();
  await expect(a.getByRole("list", { name: "Participants" })).toContainText("You");
  await b.getByRole("button", { name: "Join huddle (1)" }).click(); // live, from the stream
  await expect(a.getByRole("list", { name: "Participants" })).toContainText("Bobur");
  await expect(b.getByRole("list", { name: "Participants" })).toContainText("Ali");

  await a.getByRole("button", { name: "Start camera" }).click();
  await playing(b, "Ali's camera"); // frames crossed the peer connection
  await b.getByRole("button", { name: "Start camera" }).click();
  await playing(a, "Bobur's camera");

  await a.getByRole("button", { name: "Mute" }).click();
  await expect(b.getByRole("list", { name: "Participants" }).getByLabel("muted")).toBeVisible();

  await a.getByRole("button", { name: "Share screen" }).click();
  await expect(b.getByText("Ali is presenting")).toBeVisible();
  await playing(b, "Ali's screen");

  await b.getByRole("button", { name: "Leave" }).click();
  await expect(a.getByRole("list", { name: "Participants" })).not.toContainText("Bobur");
  await a.getByRole("button", { name: "Leave" }).click();
  await expect(b.getByRole("button", { name: "Start huddle" })).toBeVisible();
});
