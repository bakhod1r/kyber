import { expect, test } from "@playwright/test";

// C8a: the administration panel is for administrators only.
test("admin panel is hidden from regular users", async ({ page }) => {
  const id = Date.now().toString(36).toLowerCase().slice(-5);
  await page.goto("/signup");
  await page.getByLabel("Name").fill("Regular User");
  await page.getByLabel("Email").fill(`regular-${id}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByRole("link", { name: "Time table" })).toBeVisible();
  await expect(page.getByRole("link", { name: "Admin", exact: true })).toHaveCount(0);
  await page.goto("/admin");
  await expect(page.getByRole("alert")).toContainText("Only workspace administrators can see this page.");
});
