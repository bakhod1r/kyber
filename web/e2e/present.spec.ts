import { type APIRequestContext, expect, test } from "@playwright/test";

// S60: present a doc and a whiteboard to everyone in a huddle; the others follow the presenter's scrolling.
test.use({ launchOptions: { executablePath: process.env.KYBER_CHROMIUM || undefined, args: ["--use-fake-ui-for-media-stream", "--use-fake-device-for-media-stream"] } });

test("present to everyone in a huddle and follow the presenter", async ({ browser }) => {
  const id = Date.now().toString(36).slice(-5);
  const key = `P${id.toUpperCase()}`.slice(0, 6);
  const signIn = async (who: string) => {
    const ctx = await browser.newContext({ permissions: ["microphone"], viewport: { width: 1300, height: 900 } });
    const call = (r: APIRequestContext, method: string, path: string, data?: unknown) =>
      r.fetch(path, { method, data, headers: { "Content-Type": "application/json" } }).then((x) => (x.status() === 204 ? null : x.json()));
    const email = `${who}-${id}@example.com`;
    await call(ctx.request, "POST", "/api/v1/auth/signup", { email, name: who, password: "a very long password" });
    await call(ctx.request, "POST", "/api/v1/auth/login", { email, password: "a very long password" });
    return { ctx, email, call: (m: string, p: string, d?: unknown) => call(ctx.request, m, p, d) };
  };
  const ali = await signIn("Ali");
  const bobur = await signIn("Bobur");
  await ali.call("POST", "/api/v1/projects", { key, name: "Present" });
  await ali.call("POST", `/api/v1/projects/${key}/members`, { email: bobur.email, role: "member" });
  const body = Array.from({ length: 80 }, (_, i) => `Paragraph ${i + 1} of the design.`).join("\n\n");
  await ali.call("POST", `/api/v1/projects/${key}/docs`, { title: "Design notes", body });
  const channel = await ali.call("POST", "/api/v1/channels", { name: `review-${id}` });
  await bobur.call("POST", `/api/v1/conversations/${channel.id}/join`);
  await ali.call("POST", `/api/v1/conversations/${channel.id}/whiteboards`, { title: "Sketch" });

  const a = await ali.ctx.newPage();
  const b = await bobur.ctx.newPage();
  await a.goto(`/chat/${channel.id}`);
  await b.goto(`/chat/${channel.id}`);
  await a.getByRole("button", { name: "Start huddle" }).click();
  await b.getByRole("button", { name: "Join huddle (1)" }).click();
  await expect(a.getByRole("list", { name: "Participants" })).toContainText("Bobur");

  await a.getByLabel("Present to everyone").selectOption({ label: `📄 ${key}: Design notes` });
  await expect(b.getByText("Ali is presenting")).toBeVisible();
  await expect(b.getByRole("article", { name: "Doc Design notes" })).toContainText("Paragraph 1 of the design.");

  // Ali scrolls to the end; Bobur's view follows.
  await a.getByTestId("presentation-scroll").evaluate((el) => el.scrollTo({ top: el.scrollHeight }));
  await expect.poll(() => b.getByTestId("presentation-scroll").evaluate((el) => el.scrollTop / (el.scrollHeight - el.clientHeight))).toBeGreaterThan(0.9);

  // Bobur takes over with the whiteboard.
  await b.getByLabel("Present to everyone").selectOption({ label: "🖍 Sketch" });
  await expect(a.getByText("Bobur is presenting")).toBeVisible();
  await expect(a.getByRole("figure", { name: "Presentation" }).getByRole("img", { name: "Canvas" })).toBeVisible();
  await b.getByRole("button", { name: "Stop presenting" }).click();
  await expect(a.getByRole("figure", { name: "Presentation" })).toHaveCount(0);
});
