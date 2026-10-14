import { expect, test } from "@playwright/test";

// F5: a project admin adds a signed outgoing webhook, sees its secret once, and removes it.
test("a project admin manages outgoing webhooks", async ({ page }) => {
  const id = Date.now().toString(36).toUpperCase().slice(-5);
  const key = `J${id}`.slice(0, 6);
  await page.goto("/signup");
  await page.getByLabel("Name").fill("Hook Admin");
  await page.getByLabel("Email").fill(`hooks-${id.toLowerCase()}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByLabel("Key").fill(key);
  await page.getByLabel("Project name").fill("Hooks");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));

  await page.getByRole("link", { name: "Webhooks", exact: true }).click();
  await expect(page.getByText("No webhooks yet.")).toBeVisible();
  await page.getByLabel("Payload URL").fill("https://ci.example.com/kyber");
  await page.getByLabel(/Events/).fill("issue.created");
  await page.getByRole("button", { name: "Add webhook" }).click();
  await expect(page.getByRole("status")).toContainText("https://ci.example.com/kyber");
  const list = page.getByRole("list", { name: "Webhooks" });
  await expect(list).toContainText("issue.created");

  await page.getByRole("button", { name: "Delete webhook https://ci.example.com/kyber" }).click();
  await expect(page.getByText("No webhooks yet.")).toBeVisible();
});
