import { expect, test } from "@playwright/test";

// E2: a meeting for a whole project lands on the time table with its audience.
test("team event for a project", async ({ page }) => {
  const id = Date.now().toString(36).toUpperCase().slice(-5);
  const key = `E${id}`.slice(0, 6);
  await page.goto("/signup");
  await page.getByLabel("Name").fill("Event Host");
  await page.getByLabel("Email").fill(`events-${id.toLowerCase()}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByLabel("Key").fill(key);
  await page.getByLabel("Project name").fill("Events");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));

  await page.getByRole("link", { name: "Time table" }).click();
  // Wait for the page: until then getByLabel("Title") also matches the board's "New issue title".
  await expect(page.getByRole("heading", { name: "Time table" })).toBeVisible();
  const tomorrow = new Date(Date.now() + 24 * 3600_000);
  const local = new Date(tomorrow.getTime() - tomorrow.getTimezoneOffset() * 60_000).toISOString().slice(0, 11) + "15:00";
  await page.getByLabel("Title", { exact: true }).fill("Sprint review");
  await page.getByLabel("Starts").fill(local);
  await page.getByLabel("Invite").selectOption(`project:${key}`);
  await expect(page.getByText(/reminded 15 minutes before/)).toBeVisible();
  await page.getByRole("button", { name: "Schedule" }).click();
  const table = page.getByRole("list", { name: "Meetings" });
  await expect(table.getByText("Sprint review")).toBeVisible();
  await expect(table.getByText(key, { exact: true })).toBeVisible();
});
