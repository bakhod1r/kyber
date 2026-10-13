import { expect, test } from "@playwright/test";

// D1/D2: Summary, List and quick filters on the active sprints board.
test("summary, list and quick filters", async ({ page }) => {
  const id = Date.now().toString(36).toUpperCase().slice(-5);
  const key = `V${id}`.slice(0, 6);
  await page.goto("/signup");
  await page.getByLabel("Name").fill("Viewer Of Things");
  await page.getByLabel("Email").fill(`views-${id.toLowerCase()}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByLabel("Key").fill(key);
  await page.getByLabel("Project name").fill("Views");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));
  await page.getByLabel("New issue title").fill("A bug");
  await page.getByLabel("Type").selectOption("bug");
  await page.getByRole("button", { name: "Add issue" }).click();
  await expect(page.getByText("A bug")).toBeVisible();
  await page.getByLabel("New issue title").fill("A task");
  await page.getByLabel("Type").selectOption("task");
  await page.getByRole("button", { name: "Add issue" }).click();
  await expect(page.getByText("A task")).toBeVisible();

  // Quick filter on the board.
  await page.getByRole("group", { name: "Quick filters" }).getByRole("button", { name: "Bugs" }).click();
  await expect(page.getByText("A task")).toBeHidden();
  await expect(page.getByText("A bug")).toBeVisible();
  await page.getByRole("button", { name: "Clear filters" }).click();
  await expect(page.getByText("A task")).toBeVisible();

  await page.getByRole("link", { name: "List", exact: true }).click();
  const list = page.getByRole("region", { name: "Issue list" });
  await expect(list).toContainText("2 issues");
  await list.getByRole("button", { name: "Bugs" }).click();
  await expect(list).toContainText("1 issue");

  await page.getByRole("link", { name: "Summary", exact: true }).click();
  await expect(page.getByRole("region", { name: "Last 7 days" })).toContainText("Unassigned");
  await expect(page.getByRole("figure", { name: "Types of work" })).toBeVisible();
});
