import { expect, test } from "@playwright/test";

// C7: a change made in one session appears in another without reloading.
test("the board updates live", async ({ page, context }) => {
  const id = Date.now().toString(36).toUpperCase().slice(-5);
  const key = `L${id}`.slice(0, 6);
  await page.goto("/signup");
  await page.getByLabel("Name").fill("Live Watcher");
  await page.getByLabel("Email").fill(`live-${id.toLowerCase()}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByLabel("Key").fill(key);
  await page.getByLabel("Project name").fill("Live");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));
  await page.getByRole("link", { name: "List", exact: true }).click();
  await expect(page.getByRole("region", { name: "Issue list" })).toContainText("0 issues");

  // Another tab of the same user files an issue through the API.
  const other = await context.newPage();
  await other.goto("/");
  const res = await other.request.post(`/api/v1/projects/${key}/issues`, { data: { title: "Arrived live", type: "task" } });
  expect(res.status()).toBe(201);

  await expect(page.getByText("Arrived live")).toBeVisible({ timeout: 10_000 });
});
