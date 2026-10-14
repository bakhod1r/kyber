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
  const board = await ali.call("POST", `/api/v1/conversations/${channel.id}/whiteboards`, { title: "Architecture" });

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

  // Text is typed in place on the canvas (S63), and edited again by double-clicking it.
  await a.getByRole("button", { name: "Text" }).click();
  await a.mouse.click(boxA.x + 100, boxA.y + 300);
  await a.getByRole("textbox", { name: "Text" }).fill("Gateway");
  await a.keyboard.press("Enter");
  await expect(cb.getByText("Gateway")).toBeVisible();
  await a.getByRole("button", { name: "Select" }).click();
  await a.getByRole("img", { name: "Canvas" }).getByText("Gateway").dblclick({ force: true });
  await a.getByRole("textbox", { name: "Text" }).fill("API gateway");
  await a.keyboard.press("Enter");
  await expect(cb.getByText("API gateway")).toBeVisible();

  // A sticky note becomes an issue in the project, and the note links to it.
  await ali.call("POST", `/api/v1/whiteboards/${board.id}/ops`, { ops: [{ op: "add", element: {
    id: "note", kind: "sticky", x: 1200, y: 600, w: 200, h: 120, text: "Cache the board\nUse Redis", color: "#fde68a", points: [], version: 0 } }] });
  await expect(ca.getByText("Cache the board")).toBeVisible();
  await a.getByRole("button", { name: "Select" }).click();
  const scale = boxA.width / 1600;
  await a.mouse.click(boxA.x + 1300 * scale, boxA.y + 660 * scale);
  await a.getByLabel("Create issue from sticky note").selectOption(key);
  await expect(cb.getByText(`→ ${key}-1`)).toBeVisible();
  const issue = await ali.call("GET", `/api/v1/issues/${key}-1`);
  expect(issue.title).toBe("Cache the board");
  expect(issue.description).toBe("Use Redis");
  await expect(a.getByRole("link", { name: `Open ${key}-1` })).toBeVisible();

  // S86: Ali comments on the sticky note (still selected); Bobur sees the pin live.
  await a.getByLabel("Comment", { exact: true }).fill("Redis or in-process?");
  await a.getByRole("button", { name: "Comment", exact: true }).click();
  await expect(a.getByRole("complementary", { name: "Element comments" })).toContainText("Redis or in-process?");
  await expect(cb.getByLabel("1 comments on an element")).toBeVisible();
});

// S89: the board is infinite — wheel zooms around the pointer, the Hand tool pans, Fit brings everything back.
test("zoom and pan the whiteboard", async ({ page }) => {
  const id = Date.now().toString(36).slice(-5);
  await page.setViewportSize({ width: 1400, height: 1000 });
  const call = (method: string, path: string, data?: unknown) =>
    page.request.fetch(path, { method, data, headers: { "Content-Type": "application/json" } }).then((x) => (x.status() === 204 ? null : x.json()));
  const email = `zoom-${id}@example.com`;
  await call("POST", "/api/v1/auth/signup", { email, name: "Zoe", password: "a very long password" });
  await call("POST", "/api/v1/auth/login", { email, password: "a very long password" });
  const channel = await call("POST", "/api/v1/channels", { name: `zoom-${id}` });
  await call("POST", `/api/v1/conversations/${channel.id}/whiteboards`, { title: "Map" });
  await page.goto(`/chat/${channel.id}`);
  await page.getByRole("button", { name: "Whiteboards" }).click();
  await page.getByRole("button", { name: "Map" }).click();
  const canvas = page.getByRole("img", { name: "Canvas" });
  const zoom = page.getByRole("group", { name: "Zoom" });

  await page.getByRole("button", { name: "Rectangle" }).click();
  const c = (await canvas.boundingBox())!;
  await page.mouse.move(c.x + 200, c.y + 200);
  await page.mouse.down();
  await page.mouse.move(c.x + 300, c.y + 260, { steps: 4 });
  await page.mouse.up();
  const rect = canvas.locator("rect[data-id]");
  await expect(rect).toHaveCount(1);
  const before = (await rect.boundingBox())!;

  await page.mouse.move(c.x + 250, c.y + 230); // inside the rectangle
  await page.mouse.wheel(0, -400);
  await expect(zoom.getByRole("button", { name: "Reset zoom" })).not.toHaveText("100%");
  const zoomed = (await rect.boundingBox())!;
  expect(zoomed.width).toBeGreaterThan(before.width * 1.3);
  expect(zoomed.x).toBeLessThan(c.x + 250); // the point under the pointer stays inside it
  expect(zoomed.x + zoomed.width).toBeGreaterThan(c.x + 250);

  await page.getByRole("button", { name: "Hand" }).click();
  await page.mouse.move(c.x + 600, c.y + 400);
  await page.mouse.down();
  await page.mouse.move(c.x + 700, c.y + 450, { steps: 4 });
  await page.mouse.up();
  const panned = (await rect.boundingBox())!;
  expect(panned.x - zoomed.x).toBeGreaterThan(80);
  expect(panned.y - zoomed.y).toBeGreaterThan(30);
  await expect(rect).toHaveCount(1); // the Hand never draws

  await zoom.getByRole("button", { name: "Fit to content" }).click();
  const fitted = (await rect.boundingBox())!;
  expect(fitted.x).toBeGreaterThan(c.x);
  expect(fitted.x + fitted.width).toBeLessThan(c.x + c.width);
});

// S90: resize with a handle, select several with a marquee, move them together, duplicate, nudge, undo as one step.
test("resize, multi-select and duplicate on the whiteboard", async ({ page }) => {
  const id = Date.now().toString(36).slice(-5);
  await page.setViewportSize({ width: 1400, height: 1000 });
  const call = (method: string, path: string, data?: unknown) =>
    page.request.fetch(path, { method, data, headers: { "Content-Type": "application/json" } }).then((x) => (x.status() === 204 ? null : x.json()));
  const email = `multi-${id}@example.com`;
  await call("POST", "/api/v1/auth/signup", { email, name: "Mira", password: "a very long password" });
  await call("POST", "/api/v1/auth/login", { email, password: "a very long password" });
  const channel = await call("POST", "/api/v1/channels", { name: `multi-${id}` });
  await call("POST", `/api/v1/conversations/${channel.id}/whiteboards`, { title: "Shapes" });
  await page.goto(`/chat/${channel.id}`);
  await page.getByRole("button", { name: "Whiteboards" }).click();
  await page.getByRole("button", { name: "Shapes" }).click();
  const canvas = page.getByRole("img", { name: "Canvas" });
  const c = (await canvas.boundingBox())!;
  const drawRect = async (x: number, y: number) => {
    await page.getByRole("button", { name: "Rectangle" }).click();
    await page.mouse.move(c.x + x, c.y + y);
    await page.mouse.down();
    await page.mouse.move(c.x + x + 100, c.y + y + 60, { steps: 4 });
    await page.mouse.up();
  };
  await drawRect(100, 100);
  await drawRect(300, 100);
  const rects = canvas.locator("rect[data-id]");
  await expect(rects).toHaveCount(2);

  // Select the first and drag its south-east handle.
  await page.getByRole("button", { name: "Select" }).click();
  await page.mouse.click(c.x + 150, c.y + 130);
  const handle = canvas.getByLabel("Resize se");
  const h = (await handle.boundingBox())!;
  const w0 = Number(await rects.first().getAttribute("width"));
  await page.mouse.move(h.x + h.width / 2, h.y + h.height / 2);
  await page.mouse.down();
  await page.mouse.move(h.x + 60, h.y + 40, { steps: 4 });
  await page.mouse.up();
  await expect.poll(async () => Number(await rects.first().getAttribute("width"))).toBeGreaterThan(w0 + 30);

  // A marquee around both selects them; Ctrl+D duplicates both.
  await page.mouse.move(c.x + 60, c.y + 60);
  await page.mouse.down();
  await page.mouse.move(c.x + 600, c.y + 400, { steps: 6 });
  await page.mouse.up();
  await expect(page.getByText("2 selected")).toBeVisible();
  await page.keyboard.press("Control+d");
  await expect(rects).toHaveCount(4);
  const xs = async () => (await rects.evaluateAll((els) => els.map((e) => Number(e.getAttribute("x"))))).reduce((a, b) => a + b, 0);
  const x0 = await xs();
  await page.keyboard.press("Shift+ArrowRight"); // the two selected copies move 10 px each
  await expect.poll(xs).toBe(x0 + 20);
  await page.keyboard.press("Delete");
  await expect(rects).toHaveCount(2);
  await page.getByRole("button", { name: "Undo" }).click();
  await expect(rects).toHaveCount(4);
});
