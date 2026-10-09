import { expect, test } from "@playwright/test";

// KYB-S27/S28: a sprint with estimates produces correct reports in the UI.
test("reports show KPIs, distributions, burndown and velocity", async ({ page }) => {
  const id = Date.now().toString(36).slice(-5);
  const key = `R${id.toUpperCase()}`.slice(0, 6);
  const call = async (method: string, path: string, data?: unknown) => {
    const r = await page.request.fetch(path, { method, data, headers: { "Content-Type": "application/json" } });
    return r.status() === 204 ? null : r.json();
  };
  await call("POST", "/api/v1/auth/signup", { email: `lead-${id}@example.com`, name: "Lead", password: "a very long password" });
  await call("POST", "/api/v1/auth/login", { email: `lead-${id}@example.com`, password: "a very long password" });
  await call("POST", "/api/v1/projects", { key, name: "Reports" });
  const pts = [3, 5, 2, 8];
  for (const [i, p] of pts.entries()) {
    const is = await call("POST", `/api/v1/projects/${key}/issues`, { title: `Issue ${i + 1}`, type: i % 2 ? "story" : "bug" });
    await call("PATCH", `/api/v1/issues/${is.key}`, { version: is.version, estimate: p });
  }
  const sprint = await call("POST", `/api/v1/projects/${key}/sprints`, { name: "Sprint 1" });
  for (const n of [1, 2, 3]) {
    const is = await call("GET", `/api/v1/issues/${key}-${n}`);
    await call("PATCH", `/api/v1/issues/${key}-${n}`, { version: is.version, sprint_id: sprint.id });
  }
  await page.waitForTimeout(1500); // let the relay record the planning before the start
  await call("POST", `/api/v1/sprints/${sprint.id}/start`);
  await page.waitForTimeout(300);
  await call("POST", `/api/v1/issues/${key}-1/transitions`, { to: "in_progress" });
  await call("POST", `/api/v1/issues/${key}-1/transitions`, { to: "done" });

  await page.goto(`/projects/${key}/reports`);
  const kpis = page.getByRole("region", { name: "Key figures" });
  await expect(kpis.getByText("Open issues").locator("..")).toContainText("3");
  await expect(kpis.getByText("Open story points").locator("..")).toContainText("15");
  await expect(page.getByRole("figure", { name: "Issues by type" })).toBeVisible();

  // Burndown: 10 committed, 3 done -> 7 remaining (wait for the relay).
  await expect(async () => {
    await page.reload();
    const burn = page.getByRole("figure", { name: "Burndown — Sprint 1" });
    await expect(burn).toContainText("from 10 at the start", { timeout: 1000 });
    await burn.getByText("Show data table").click();
    await expect(burn.getByRole("table")).toContainText("7", { timeout: 1000 });
  }).toPass({ timeout: 15_000 });

  await expect(page.getByText("Complete a sprint to see velocity.")).toBeVisible();
  await call("POST", `/api/v1/sprints/${sprint.id}/complete`);
  await expect(async () => {
    await page.reload();
    await expect(page.getByRole("figure", { name: "Velocity" })).toBeVisible({ timeout: 1000 });
  }).toPass({ timeout: 15_000 });
  await page.getByRole("figure", { name: "Velocity" }).getByText("Show data table").click();
  await expect(page.getByRole("figure", { name: "Velocity" }).getByRole("table")).toContainText("Sprint 1");
});
