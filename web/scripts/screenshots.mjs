// Regenerates the README screenshots in docs/assets against a running Kyber
// (in-memory mode is fine): `make screenshots`.
import { chromium } from "@playwright/test";

const base = process.env.KYBER_URL ?? "http://localhost:18080";
const out = new URL("../../docs/assets/", import.meta.url).pathname;
const browser = await chromium.launch({
  executablePath: process.env.KYBER_CHROMIUM || undefined,
  args: ["--use-fake-ui-for-media-stream", "--use-fake-device-for-media-stream"], // huddle cameras
});

async function user(email, name) {
  const ctx = await browser.newContext({ viewport: { width: 1360, height: 820 }, deviceScaleFactor: 2, baseURL: base, permissions: ["microphone", "camera"] });
  const r = await ctx.request.post("/api/v1/auth/signup", { data: { email, name, password: "a very long password" } });
  if (!r.ok()) throw new Error(`signup ${email}: ${r.status()}`);
  await ctx.request.post("/api/v1/auth/login", { data: { email, password: "a very long password" } });
  const api = async (method, path, data) => {
    const res = await ctx.request.fetch(path, { method, data });
    if (!res.ok()) throw new Error(`${method} ${path}: ${res.status()} ${await res.text()}`);
    return res.status() === 204 ? null : res.json();
  };
  return { ctx, api, me: await api("GET", "/api/v1/me") };
}

const lead = await user("aziza@kyber.dev", "Aziza Karimova");
const dev = await user("timur@kyber.dev", "Timur Rashidov");
const qa = await user("malika@kyber.dev", "Malika Usmonova");
const { api } = lead;

await api("POST", "/api/v1/projects", { key: "KYB", name: "Kyber Platform" });
await api("POST", "/api/v1/projects/KYB/members", { email: "timur@kyber.dev", role: "member" });
await api("POST", "/api/v1/projects/KYB/members", { email: "malika@kyber.dev", role: "member" });
await api("PUT", "/api/v1/projects/KYB/workflow", {
  initial: "todo",
  statuses: [
    { id: "todo", name: "To Do", category: "todo" },
    { id: "in_progress", name: "In Progress", category: "in_progress" },
    { id: "code_review", name: "Code Review", category: "in_progress" },
    { id: "done", name: "Done", category: "done" },
  ],
  transitions: [],
});

const sprint = await api("POST", "/api/v1/projects/KYB/sprints", { name: "Sprint 12", goal: "Ship configurable workflows" });
const issues = [
  ["Configurable workflows per project", "story", "highest", 8, dev, "done"],
  ["Board columns follow the workflow", "story", "high", 5, dev, "code_review"],
  ["Workflow editor: add, rename, remove statuses", "task", "high", 5, qa, "in_progress"],
  ["Login with Telegram bot code", "story", "medium", 3, lead, "done"],
  ["Burndown ignores re-opened issues", "bug", "highest", 2, qa, "in_progress"],
  ["WIP limits turn the column red", "task", "medium", 3, dev, "todo"],
  ["Import Jira CSV with custom statuses", "task", "low", 2, null, "todo"],
  ["Mobile layout for the board", "bug", "medium", 1, qa, "todo"],
];
const created = [];
for (const [title, type, priority, estimate, who, status] of issues) {
  let is = await api("POST", "/api/v1/projects/KYB/issues", { title, type });
  is = await api("PATCH", `/api/v1/issues/${is.key}`, {
    version: is.version, priority, estimate, assignee_id: who ? who.me.id : null, sprint_id: sprint.id,
    description: `${title}. Acceptance criteria agreed in planning; see the linked ADR.`,
  });
  // Done issues pass through In Progress, as real work does (cycle time needs both).
  if (status === "done") await api("POST", `/api/v1/issues/${is.key}/transitions`, { to: "in_progress" });
  if (status !== "todo") await api("POST", `/api/v1/issues/${is.key}/transitions`, { to: status });
  created.push(is);
}
for (const [title, type] of [["Admin panel for superusers", "epic"], ["Attachments on MinIO", "story"], ["Realtime board over SSE", "story"]]) {
  await api("POST", "/api/v1/projects/KYB/issues", { title, type });
}
await api("POST", `/api/v1/sprints/${sprint.id}/start`, {});
await api("POST", "/api/v1/projects/KYB/sprints", { name: "Sprint 13", goal: "Boards and WIP limits" });
await dev.api("POST", `/api/v1/issues/${created[1].key}/comments`, { body: "Columns now come from the project workflow. @Aziza Karimova could you review?" });
await qa.api("POST", `/api/v1/issues/${created[1].key}/comments`, { body: "Verified on Chrome and Firefox, drag and drop works across the new Code Review column." });
const at = new Date(Date.now() + 26 * 3600e3);
at.setUTCMinutes(0, 0, 0);
await api("POST", "/api/v1/meetings", { title: "Sprint 12 review", starts_at: at.toISOString(), ends_at: new Date(+at + 3600e3).toISOString(), attendee_ids: [dev.me.id, qa.me.id] });
await api("POST", "/api/v1/meetings", { title: "Backlog refinement", starts_at: new Date(+at + 3 * 3600e3).toISOString(), ends_at: new Date(+at + 4 * 3600e3).toISOString(), attendee_ids: [dev.me.id] });
await new Promise((r) => setTimeout(r, 1500)); // outbox relay → notifications and reports

const page = await lead.ctx.newPage();
const shot = async (name, path, ready) => {
  await page.goto(path);
  await ready();
  await page.waitForTimeout(400);
  await page.screenshot({ path: `${out}${name}.png` });
  console.log(`docs/assets/${name}.png`);
};
const anon = await (await browser.newContext({ viewport: { width: 1360, height: 820 }, deviceScaleFactor: 2, baseURL: base })).newPage();
await anon.goto("/");
await anon.getByRole("heading", { level: 1 }).waitFor();
await anon.screenshot({ path: `${out}landing.png` });
console.log("docs/assets/landing.png");

await shot("board", "/projects/KYB", () => page.getByRole("region", { name: "Code Review" }).getByText("Board columns").waitFor());
await shot("workflow", "/projects/KYB/workflow", () => page.getByRole("button", { name: "Save workflow" }).waitFor());
await shot("backlog", "/projects/KYB/backlog", () => page.locator("text=Sprint 13 >> visible=true").first().waitFor());
await shot("issue-panel", `/projects/KYB?issue=${created[1].key}`, async () => {
  await page.getByLabel("Comments").getByText("Verified on Chrome").waitFor();
  await page.getByRole("heading", { name: "Comments" }).evaluate((el) => el.scrollIntoView({ block: "center" }));
});
await shot("reports", "/projects/KYB/reports", () => page.getByText(/velocity/i).first().waitFor());
await shot("meetings", "/meetings", () => page.getByText("Sprint 12 review").waitFor());
await page.goto("/projects/KYB");
await page.getByRole("button", { name: /Notifications/ }).click();
await page.getByRole("dialog", { name: "Notifications" }).waitFor();
await page.waitForTimeout(400);
await page.screenshot({ path: `${out}notifications.png` });
console.log("docs/assets/notifications.png");

// Chat, whiteboard and a recorded video huddle (S50–S62).
const general = await api("POST", "/api/v1/channels", { name: "general", topic: "Team-wide announcements" });
await api("POST", "/api/v1/channels", { name: "backend" });
await dev.api("POST", `/api/v1/conversations/${general.id}/join`, {});
await api("POST", `/api/v1/conversations/${general.id}/messages`, { body: "Good morning team! Release planning at 11:00" });
const ready = await dev.api("POST", `/api/v1/conversations/${general.id}/messages`, { body: `${created[1].key} is ready for review @aziza@kyber.dev` });
await api("POST", `/api/v1/messages/${ready.id}/reactions`, { emoji: "🎉" });
await api("POST", `/api/v1/messages/${ready.id}/replies`, { body: "Looking now" });
await api("POST", `/api/v1/conversations/${general.id}/whiteboards`, { title: "Architecture" });
await shot("chat", `/chat/${general.id}`, async () => {
  await page.getByRole("button", { name: "1 reply" }).click();
  await page.getByText("Looking now").waitFor();
});
await shot("whiteboard", `/chat/${general.id}`, async () => {
  await page.getByRole("button", { name: "Whiteboards" }).click();
  await page.getByRole("button", { name: "Architecture" }).click();
  const c = await page.getByRole("img", { name: "Canvas" }).boundingBox();
  const draw = async (tool, x1, y1, x2, y2) => {
    await page.getByRole("button", { name: tool }).click();
    await page.mouse.move(c.x + x1, c.y + y1);
    await page.mouse.down();
    await page.mouse.move(c.x + x2, c.y + y2, { steps: 5 });
    await page.mouse.up();
  };
  await draw("Rectangle", 80, 80, 260, 160);
  await draw("Rectangle", 420, 80, 600, 160);
  await draw("Ellipse", 250, 260, 430, 340);
  await draw("Arrow", 260, 120, 420, 120);
  await draw("Arrow", 510, 160, 400, 270);
});
const other = await dev.ctx.newPage();
await shot("huddle", `/chat/${general.id}`, async () => {
  await page.getByRole("button", { name: "Start huddle" }).click();
  await other.goto(`/chat/${general.id}`);
  await other.getByRole("button", { name: "Join huddle (1)" }).click();
  await page.getByRole("button", { name: "Start camera" }).click();
  await other.getByRole("button", { name: "Start camera" }).click();
  await page.getByRole("button", { name: "Record" }).click();
  await page.waitForTimeout(2500);
});
await browser.close();
