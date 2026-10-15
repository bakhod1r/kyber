import { type APIRequestContext, expect, test } from "@playwright/test";

// S58: two Kyber replicas on one PostgreSQL — Ali on one, Bobur on the other; chat and huddles still meet.
// Runs when KYBER_E2E_URL2 points at a second replica.
const second = process.env.KYBER_E2E_URL2;
test.skip(!second, "set KYBER_E2E_URL2 to a second replica");
test.use({ launchOptions: { executablePath: process.env.KYBER_CHROMIUM || undefined, args: ["--use-fake-ui-for-media-stream", "--use-fake-device-for-media-stream"] } });

test("chat and a huddle across two replicas", async ({ browser, baseURL }) => {
  const id = Date.now().toString(36).slice(-5);
  const key = `R${id.toUpperCase()}`.slice(0, 6);
  const signIn = async (who: string, url: string) => {
    const ctx = await browser.newContext({ baseURL: url, permissions: ["camera", "microphone"] });
    const call = (r: APIRequestContext, method: string, path: string, data?: unknown) =>
      r.fetch(path, { method, data, headers: { "Content-Type": "application/json" } }).then((x) => (x.status() === 204 ? null : x.json()));
    const email = `${who}-${id}@example.com`;
    await call(ctx.request, "POST", "/api/v1/auth/signup", { email, name: who, password: "a very long password" });
    await call(ctx.request, "POST", "/api/v1/auth/login", { email, password: "a very long password" });
    return { ctx, email, call: (m: string, p: string, d?: unknown) => call(ctx.request, m, p, d) };
  };
  const ali = await signIn("Ali", baseURL!);
  const bobur = await signIn("Bobur", second!);
  await ali.call("POST", "/api/v1/projects", { key, name: "Replicas" });
  await ali.call("POST", `/api/v1/projects/${key}/members`, { email: bobur.email, role: "member" });
  const channel = await ali.call("POST", "/api/v1/channels", { name: `rep-${id}` });
  await bobur.call("POST", `/api/v1/conversations/${channel.id}/join`);

  const a = await ali.ctx.newPage();
  const b = await bobur.ctx.newPage();
  await a.goto(`/chat/${channel.id}`);
  await b.goto(`/chat/${channel.id}`);
  const box = a.getByRole("textbox", { name: `Message # rep-${id}` });
  await box.fill("hello from replica one");
  await box.press("Enter");
  await expect(b.getByRole("log", { name: "Messages" })).toContainText("hello from replica one");

  await a.getByRole("button", { name: "Start huddle" }).click();
  await b.getByRole("button", { name: "Join huddle (1)" }).click();
  await a.getByRole("button", { name: "Start camera" }).click();
  await expect
    .poll(() => b.getByLabel("Ali's camera").evaluate((v: HTMLVideoElement) => v.videoWidth * (v.readyState >= 2 ? 1 : 0)), { timeout: 15_000 })
    .toBeGreaterThan(0); // the signaling crossed replicas (large events take the spill table: unit-tested)
});

// S105: presence is shared — Ali's open tab on replica one shows him online to Bobur on replica two,
// and closing it takes him offline there too.
test("presence across two replicas", async ({ browser, baseURL }) => {
  const id = Date.now().toString(36).slice(-5);
  const signIn = async (who: string, url: string) => {
    const ctx = await browser.newContext({ baseURL: url });
    const email = `${who}-${id}@example.com`;
    const post = (path: string, data: unknown) => ctx.request.post(path, { data, headers: { "Content-Type": "application/json" } });
    await post("/api/v1/auth/signup", { email, name: who, password: "a very long password" });
    await post("/api/v1/auth/login", { email, password: "a very long password" });
    const me = await (await ctx.request.get("/api/v1/me")).json();
    return { ctx, id: me.id as string };
  };
  const ali = await signIn("AliP", baseURL!);
  const bobur = await signIn("BoburP", second!);
  const online = async () => (await (await bobur.ctx.request.get(`/api/v1/presence?user_id=${ali.id}`)).json()).online as string[];
  expect(await online()).toEqual([]);

  const page = await ali.ctx.newPage();
  await page.goto("/"); // the app opens its MQTT connection on replica one
  await expect.poll(online, { timeout: 20_000 }).toEqual([ali.id]);
  await page.close();
  await expect.poll(online, { timeout: 20_000 }).toEqual([]);
});
