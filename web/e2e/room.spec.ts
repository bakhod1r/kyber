import { type APIRequestContext, type Page, expect, test } from "@playwright/test";

// S99 (ADR-0017): with a media server (LiveKit) a huddle takes more than six people and the media goes through it —
// no browser-to-browser signaling. Runs when KYBER_E2E_ROOM_URL points at a Kyber with KYBER_LIVEKIT_* set
// (`make e2e-rooms`).
const base = process.env.KYBER_E2E_ROOM_URL;
test.skip(!base, "set KYBER_E2E_ROOM_URL to a server with a media server");
test.use({
  baseURL: base,
  launchOptions: {
    executablePath: process.env.KYBER_CHROMIUM || undefined,
    args: ["--use-fake-ui-for-media-stream", "--use-fake-device-for-media-stream", "--auto-accept-this-tab-capture", "--auto-select-desktop-capture-source=Entire screen"],
  },
});

async function playing(page: Page, label: string) {
  await expect
    .poll(() => page.getByLabel(label).evaluate((v: HTMLVideoElement) => v.videoWidth * (v.readyState >= 2 ? 1 : 0)), { timeout: 20_000 })
    .toBeGreaterThan(0);
}

test("seven people in a huddle through the media server", async ({ browser }) => {
  test.setTimeout(60_000);
  const id = Date.now().toString(36).slice(-5);
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
  const key = `R${id.toUpperCase()}`.slice(0, 6);
  await ali.call("POST", "/api/v1/projects", { key, name: "Rooms" }); // names come from shared projects
  await ali.call("POST", `/api/v1/projects/${key}/members`, { email: bobur.email, role: "member" });
  const channel = await ali.call("POST", "/api/v1/channels", { name: `all-${id}` });
  await bobur.call("POST", `/api/v1/conversations/${channel.id}/join`);

  const a = await ali.ctx.newPage();
  const b = await bobur.ctx.newPage();
  const signals: string[] = [];
  for (const p of [a, b]) p.on("request", (r) => r.url().endsWith("/signal") && signals.push(r.url()));
  await a.goto(`/chat/${channel.id}`);
  await b.goto(`/chat/${channel.id}`);
  await a.getByRole("button", { name: "Start huddle" }).click();
  await expect(a.getByRole("list", { name: "Participants" })).toContainText("You");

  // Five more people join (without media) — past the mesh limit of six.
  for (let i = 0; i < 5; i++) {
    const p = await signIn(`P${i}`);
    await p.call("POST", `/api/v1/conversations/${channel.id}/join`);
    expect((await p.call("POST", `/api/v1/conversations/${channel.id}/call`)).room).toBe(true);
  }
  await b.getByRole("button", { name: "Join huddle (6)" }).click();
  await expect(a.getByRole("list", { name: "Participants" })).toContainText("Bobur");

  await a.getByRole("button", { name: "Start camera" }).click();
  await playing(b, "Ali's camera"); // through LiveKit
  await b.getByRole("button", { name: "Share screen" }).click();
  await expect(a.getByText("Bobur is presenting")).toBeVisible();
  await playing(a, "Bobur's screen");
  expect(signals).toEqual([]);

  // S103: the host removes Bobur; his browser is told and leaves the room.
  await expect(b.getByRole("button", { name: /^Remove / })).toHaveCount(0); // only the host may
  await a.getByRole("button", { name: "Remove Bobur from the huddle" }).click();
  await expect(a.getByRole("list", { name: "Participants" })).not.toContainText("Bobur");
  await expect(b.getByText("You were removed from the huddle")).toBeVisible();
  await expect(a.getByLabel("Bobur's screen")).toHaveCount(0);
  await a.getByRole("button", { name: "Leave" }).click();
});

// S104: with LiveKit Egress, Record in a room call is done by the media server; the MP4 lands in the channel.
test("the media server records a room call", async ({ browser }) => {
  test.skip(!process.env.KYBER_E2E_EGRESS, "set KYBER_E2E_EGRESS with a server whose LiveKit has Egress");
  test.setTimeout(120_000);
  const id = Date.now().toString(36).slice(-5);
  const ctx = await browser.newContext({ permissions: ["camera", "microphone"] });
  const api = (method: string, path: string, data?: unknown) =>
    ctx.request.fetch(path, { method, data, headers: { "Content-Type": "application/json" } }).then((x) => (x.status() === 204 ? null : x.json()));
  const email = `rec-${id}@example.com`;
  await api("POST", "/api/v1/auth/signup", { email, name: "Rec", password: "a very long password" });
  await api("POST", "/api/v1/auth/login", { email, password: "a very long password" });
  const channel = await api("POST", "/api/v1/channels", { name: `rec-${id}` });
  const page = await ctx.newPage();
  await page.goto(`/chat/${channel.id}`);
  await page.getByRole("button", { name: "Start huddle" }).click();
  await page.getByRole("button", { name: "Start camera" }).click();
  await page.getByRole("button", { name: "Record" }).click();
  await expect(page.getByText(/Recording by/)).toBeVisible();
  await page.waitForTimeout(8_000); // something worth recording
  await page.getByRole("button", { name: "Stop recording" }).click();
  await expect(page.getByText("Saving the recording…")).toHaveCount(0); // no browser upload
  let file: { id: string; name: string; size: number } | undefined;
  await expect
    .poll(async () => {
      const msgs = await api("GET", `/api/v1/conversations/${channel.id}/messages`);
      const rec = msgs.items.find((m: { kind: string }) => m.kind === "huddle_recording");
      file = rec && { id: rec.id, ...rec.file };
      return file?.name ?? "";
    }, { timeout: 90_000 })
    .toMatch(/^Huddle .*\.mp4$/);
  expect(file!.size).toBeGreaterThan(10_000);
  const res = await ctx.request.get(`/api/v1/messages/${file!.id}/file`);
  expect(res.status()).toBe(200);
  expect((await res.body()).subarray(4, 8).toString()).toBe("ftyp"); // an MP4
  await page.getByRole("button", { name: "Leave" }).click();
});
