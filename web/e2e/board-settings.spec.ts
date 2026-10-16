import { expect, test } from "@playwright/test";

// S44: an admin configures the board's columns, status mapping and WIP limits; the board follows.
test("a project admin configures the board", async ({ page }) => {
  const id = Date.now().toString(36).toUpperCase().slice(-5);
  const key = `B${id}`.slice(0, 6);
  await page.goto("/signup");
  await page.getByLabel("Name").fill("Board Admin");
  await page.getByLabel("Email").fill(`board-${id.toLowerCase()}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByLabel("Key").fill(key);
  await page.getByLabel("Project name").fill("Boards");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));
  await page.getByLabel("New issue title").fill("Ship it");
  await page.getByRole("button", { name: "Add issue" }).click();
  await expect(page.getByText("Ship it")).toBeVisible();

  await page.getByRole("link", { name: "Settings", exact: true }).click();

  await page.getByRole("link", { name: "Board settings", exact: true }).click();
  const settings = page.getByRole("region", { name: "Board settings" });
  await settings.getByRole("button", { name: "Add column" }).click();
  await settings.getByLabel("Column 4 name").fill("Ready");
  await settings.getByRole("button", { name: "Move Ready left" }).click();
  await settings.getByRole("button", { name: "Move Ready left" }).click();
  await settings.getByRole("button", { name: "Move Ready left" }).click();
  await settings.getByLabel("Column for To Do").selectOption({ label: "Ready" });
  await settings.getByLabel("Ready max").fill("1");
  await settings.getByRole("button", { name: "Save board" }).click();
  await expect(settings.getByRole("status")).toHaveText("Board saved");

  await page.getByRole("link", { name: "Active sprints", exact: true }).click();
  const ready = page.getByRole("region", { name: "Ready" });
  await expect(ready.getByText("Ship it")).toBeVisible();
  await expect(ready).toContainText("1 / 1");
  await expect(page.getByRole("region")).toHaveCount(4); // Ready, To Do (now empty), In Progress, Done
});
