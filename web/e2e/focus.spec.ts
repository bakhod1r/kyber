import { expect, test } from "@playwright/test";

// KYB-S41/S42: a meeting blocks a Pomodoro; once it is cancelled the timer runs in the top bar.
test("meetings block Pomodoro; the timer runs and stops", async ({ page }) => {
  const id = Date.now().toString(36).slice(-5);
  const key = `F${id.toUpperCase()}`.slice(0, 6);
  const call = async (method: string, path: string, data?: unknown) => {
    const r = await page.request.fetch(path, { method, data, headers: { "Content-Type": "application/json" } });
    return r.status() === 204 ? null : r.json();
  };
  await call("POST", "/api/v1/auth/signup", { email: `f-${id}@example.com`, name: "Focus", password: "a very long password" });
  await call("POST", "/api/v1/auth/login", { email: `f-${id}@example.com`, password: "a very long password" });
  await call("POST", "/api/v1/projects", { key, name: "Focus" });
  await call("POST", `/api/v1/projects/${key}/issues`, { title: "Deep work", type: "task" });
  const now = Date.now();
  const meeting = await call("POST", "/api/v1/meetings", {
    title: "Standup", starts_at: new Date(now + 5 * 60_000).toISOString(), ends_at: new Date(now + 20 * 60_000).toISOString(), attendee_ids: [],
  });

  await page.goto("/meetings");
  await expect(page.getByRole("list", { name: "Meetings" })).toContainText("Standup");

  await page.goto(`/projects/${key}?issue=${key}-1`);
  await page.getByRole("button", { name: "Focus 25 min" }).click();
  await expect(page.getByRole("alert")).toContainText("Standup");

  await page.goto("/meetings");
  await page.getByRole("button", { name: "Cancel Standup" }).click();
  await expect(page.getByRole("list", { name: "Meetings" })).toContainText("No meetings");
  expect(meeting.id).toBeTruthy();

  await page.goto(`/projects/${key}?issue=${key}-1`);
  await page.getByRole("button", { name: "Focus 25 min" }).click();
  const timer = page.getByRole("timer", { name: `Focus on ${key}-1` });
  await expect(timer).toContainText(/2[45]:\d\d/);
  await page.getByRole("button", { name: "Pause" }).click();
  await expect(page.getByRole("button", { name: "Resume" })).toBeVisible();
  await page.getByRole("button", { name: "Stop" }).click();
  await expect(timer).toHaveCount(0);
  await expect(page.getByRole("list", { name: "Focus history" })).toContainText("stopped");
});
