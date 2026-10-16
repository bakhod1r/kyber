import { expect, test } from "@playwright/test";

// ADR-0003 phase 2: a project admin gives the project its own permission scheme, then resets it.
test("a project admin edits the permission scheme", async ({ page }) => {
  const id = Date.now().toString(36).toUpperCase().slice(-5);
  const key = `P${id}`.slice(0, 6);

  await page.goto("/signup");
  await page.getByLabel("Name").fill("Scheme Admin");
  await page.getByLabel("Email").fill(`perm-${id.toLowerCase()}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByLabel("Key").fill(key);
  await page.getByLabel("Project name").fill("Permissions");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));

  await page.getByRole("link", { name: "Settings", exact: true }).click();

  await page.getByRole("link", { name: "Permissions", exact: true }).click();
  await expect(page.getByText(/uses the default permission scheme/)).toBeVisible();
  await expect(page.getByLabel("Administer projects: Admin")).toBeDisabled();
  await page.getByLabel("Create issues: Viewer").check();
  await page.getByRole("button", { name: "Save permissions" }).click();
  await expect(page.getByRole("status")).toHaveText("Saved");
  await page.reload();
  await expect(page.getByLabel("Create issues: Viewer")).toBeChecked();
  await expect(page.getByText(/has its own permission scheme/)).toBeVisible();

  await page.getByRole("button", { name: "Reset to default" }).click();
  await expect(page.getByText(/uses the default permission scheme/)).toBeVisible();
  await expect(page.getByLabel("Create issues: Viewer")).not.toBeChecked();
});
