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
const board = await api("POST", `/api/v1/conversations/${general.id}/whiteboards`, { title: "Architecture" });
// A styled sketch (S89–S93): filled boxes, a dashed boundary, arrows and sticky notes in colour.
const el = (id, kind, x, y, w, h, extra = {}) => ({ op: "add", element: { id, kind, x, y, w, h, points: [], text: "", color: "#1f2937", version: 0, ...extra } });
const arrow = (id, x, y, dx, dy) => el(id, "arrow", x, y, 0, 0, { points: [{ x: 0, y: 0 }, { x: dx, y: dy }], stroke: 2 });
await api("POST", `/api/v1/whiteboards/${board.id}/ops`, { ops: [
  el("vpc", "rect", 40, 40, 900, 520, { color: "#64748b", dashed: true, z: -1 }),
  el("vpc-t", "text", 56, 50, 220, 34, { text: "Production VPC", color: "#64748b" }),
  el("web", "rect", 100, 140, 200, 90, { color: "#0b6fd8", fill: true, stroke: 2, text: "Web app" }),
  el("api", "rect", 420, 140, 200, 90, { color: "#16a34a", fill: true, stroke: 2, text: "Kyber API" }),
  el("mqtt", "ellipse", 420, 360, 200, 110, { color: "#9333ea", fill: true, stroke: 2, text: "MQTT broker" }),
  el("db", "rect", 720, 140, 180, 90, { color: "#ea580c", fill: true, stroke: 3, text: "PostgreSQL" }),
  arrow("a1", 300, 185, 120, 0), arrow("a2", 620, 185, 100, 0), arrow("a3", 520, 230, 0, 130), arrow("a4", 420, 415, -220, -185),
  el("s1", "sticky", 990, 60, 200, 130, { color: "#fde68a", text: "Cache sessions?\n@timur" }),
  el("s2", "sticky", 990, 220, 200, 130, { color: "#bbf7d0", text: "Replicas share\nchat files on S3" }),
  el("s3", "sticky", 990, 380, 200, 130, { color: "#fbcfe8", text: "Load test\nbefore release" }),
] });
await shot("chat", `/chat/${general.id}`, async () => {
  await page.getByRole("button", { name: "1 reply" }).click();
  await page.getByText("Looking now").waitFor();
});
await shot("whiteboard", `/chat/${general.id}`, async () => {
  await page.getByRole("button", { name: "Whiteboards" }).click();
  await page.getByRole("button", { name: "Architecture" }).click();
  const canvas = page.getByRole("img", { name: "Canvas" });
  await canvas.getByText("Kyber API").waitFor();
  await page.getByRole("button", { name: "Fit to content" }).click();
  await canvas.locator(`rect[data-id="api"]`).click(); // selected: handles and the comment panel show
  await page.getByRole("region", { name: "Whiteboard Architecture" }).evaluate((el) => el.scrollIntoView({ block: "start" }));
});

// Docs with paragraph comments (S86).
const doc = await api("POST", "/api/v1/projects/KYB/docs", { title: "Release runbook", body: [
  "# Release runbook", "Freeze main on Thursday and cut a release branch.",
  "Run the load test against staging with 200 virtual users for 15 minutes.",
  "Tag the release, publish the images and announce it in #general.",
].join("\n\n") });
await dev.api("POST", "/api/v1/projects/KYB/members", { email: "timur@kyber.dev", role: "member" }).catch(() => {});
await api("POST", `/api/v1/docs/${doc.id}/comments`, { paragraph: 2, quote: "Run the load test against staging", body: "Should we include the MQTT broker in the test? @timur@kyber.dev" });
await api("POST", `/api/v1/docs/${doc.id}/comments`, { paragraph: 2, quote: "Run the load test against staging", body: "Yes — and replica failover." });
await shot("docs", `/projects/KYB/docs?page=${doc.id}`, async () => {
  await page.getByRole("button", { name: "2 comments on paragraph 3" }).click();
  await page.getByText("replica failover").waitFor();
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
