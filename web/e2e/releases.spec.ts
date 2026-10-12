import { expect, test } from "@playwright/test";

// S48: an admin creates a version, plans an issue into it, releases it and reads the notes.
test("a project admin plans and releases a version", async ({ page }) => {
  const id = Date.now().toString(36).toUpperCase().slice(-5);
  const key = `V${id}`.slice(0, 6);
  await page.goto("/signup");
  await page.getByLabel("Name").fill("Release Admin");
  await page.getByLabel("Email").fill(`rel-${id.toLowerCase()}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByLabel("Key").fill(key);
  await page.getByLabel("Project name").fill("Releases");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));

  await page.getByRole("link", { name: "Releases", exact: true }).click();
  await page.getByLabel("Version name").fill("1.0");
  await page.getByRole("button", { name: "Create version" }).click();
  await expect(page.getByRole("status")).toHaveText("1.0 created");

  await page.getByRole("link", { name: "Board", exact: true }).click();
  await page.getByLabel("New issue title").fill("Ship login");
  await page.getByRole("button", { name: "Add issue" }).click();
  await page.getByText("Ship login").click();
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("group", { name: "Fix versions" }).getByLabel("1.0").check();
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(dialog.getByText("Saved")).toBeVisible();
  await page.keyboard.press("Escape");

  await page.getByRole("link", { name: "Releases", exact: true }).click();
  const row = page.getByRole("row", { name: /^1\.0/ });
  await expect(row).toContainText("0/1 done");
  await row.getByRole("button", { name: "Release 1.0" }).click();
  await expect(page.getByRole("status")).toHaveText("1.0 released");
  await page.getByRole("row", { name: /^1\.0/ }).getByRole("button", { name: "Release notes" }).click();
  await expect(page.getByRole("region", { name: "Release notes" })).toContainText("Ship login");
});
