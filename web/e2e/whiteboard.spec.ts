import { type APIRequestContext, expect, type Page, test } from "@playwright/test";

// S59: two people draw on one whiteboard; shapes, moves and cursors arrive live; undo works.
test("two people draw on a whiteboard together", async ({ browser }) => {
  const id = Date.now().toString(36).slice(-5);
  const key = `W${id.toUpperCase()}`.slice(0, 6);
  const signIn = async (who: string) => {
    const ctx = await browser.newContext({ viewport: { width: 1400, height: 1000 } });
    const call = (r: APIRequestContext, method: string, path: string, data?: unknown) =>
      r.fetch(path, { method, data, headers: { "Content-Type": "application/json" } }).then((x) => (x.status() === 204 ? null : x.json()));
    const email = `${who}-${id}@example.com`;
    await call(ctx.request, "POST", "/api/v1/auth/signup", { email, name: who, password: "a very long password" });
    await call(ctx.request, "POST", "/api/v1/auth/login", { email, password: "a very long password" });
    return { ctx, email, call: (m: string, p: string, d?: unknown) => call(ctx.request, m, p, d) };
  };
  const ali = await signIn("Ali");
  const bobur = await signIn("Bobur");
  await ali.call("POST", "/api/v1/projects", { key, name: "Board" });
  await ali.call("POST", `/api/v1/projects/${key}/members`, { email: bobur.email, role: "member" });
  const channel = await ali.call("POST", "/api/v1/channels", { name: `arch-${id}` });
  await bobur.call("POST", `/api/v1/conversations/${channel.id}/join`);
  await ali.call("POST", `/api/v1/conversations/${channel.id}/whiteboards`, { title: "Architecture" });

  const open = async (page: Page) => {
    await page.goto(`/chat/${channel.id}`);
    await page.getByRole("button", { name: "Whiteboards" }).click();
    await page.getByRole("button", { name: "Architecture" }).click();
    return page.getByRole("img", { name: "Canvas" });
  };
  const a = await ali.ctx.newPage();
  const b = await bobur.ctx.newPage();
  const ca = await open(a);
  const cb = await open(b);

  // Ali draws a rectangle by dragging.
  await a.getByRole("button", { name: "Rectangle" }).click();
  const boxA = (await ca.boundingBox())!;
  await a.mouse.move(boxA.x + 100, boxA.y + 100);
  await a.mouse.down();
  await a.mouse.move(boxA.x + 250, boxA.y + 180, { steps: 5 });
  await a.mouse.up();
  await expect(cb.locator("rect[data-id]")).toHaveCount(1); // live on Bobur's screen
  await expect(cb.getByLabel("Ali's cursor")).toBeVisible();

  // Bobur moves it; Ali sees it move.
  const before = Number(await ca.locator("rect[data-id]").getAttribute("x"));
  const boxB = (await cb.boundingBox())!;
  await b.mouse.move(boxB.x + 150, boxB.y + 140);
  await b.mouse.down();
  await b.mouse.move(boxB.x + 400, boxB.y + 300, { steps: 5 });
  await b.mouse.up();
  await expect.poll(async () => Number(await ca.locator("rect[data-id]").getAttribute("x"))).toBeGreaterThan(before + 50);

  // Bobur undoes the move; both see the original position again.
  await b.getByRole("button", { name: "Undo" }).click();
  await expect.poll(async () => Number(await ca.locator("rect[data-id]").getAttribute("x"))).toBe(before);

  // A freehand stroke from Bobur.
  await b.getByRole("button", { name: "Pen" }).click();
  await b.mouse.move(boxB.x + 500, boxB.y + 100);
  await b.mouse.down();
  for (let i = 1; i <= 10; i++) await b.mouse.move(boxB.x + 500 + i * 10, boxB.y + 100 + (i % 2) * 20);
  await b.mouse.up();
  await expect(ca.locator("polyline[data-id]")).toHaveCount(1);
});
