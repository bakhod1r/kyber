import { expect, test } from "@playwright/test";

// Custom fields: an admin adds a select field, fills it in on an issue and finds the issue with JQL.
test("custom fields on issues and in JQL", async ({ page }) => {
  const id = Date.now().toString(36).toUpperCase().slice(-5);
  const key = `C${id}`.slice(0, 6);
  await page.goto("/signup");
  await page.getByLabel("Name").fill("Field Admin");
  await page.getByLabel("Email").fill(`fields-${id.toLowerCase()}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByLabel("Key").fill(key);
  await page.getByLabel("Project name").fill("Fields");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));

  await page.getByRole("link", { name: "Fields", exact: true }).click();
  const admin = page.getByRole("region", { name: "Custom fields" });
  await admin.getByLabel("Field name").fill("Tier");
  await admin.getByLabel("Field type").selectOption("select");
  await admin.getByLabel("Options").fill("Gold, Silver");
  await admin.getByRole("button", { name: "Add field" }).click();
  await expect(admin.getByLabel("Name of Tier")).toBeVisible();

  await page.getByRole("link", { name: "Board", exact: true }).click();
  await page.getByLabel("New issue title").fill("Big deal");
  await page.getByRole("button", { name: "Add issue" }).click();
  await page.getByText("Big deal").click();
  const details = page.getByRole("dialog").getByRole("region", { name: "Details" });
  await details.getByLabel("Tier").selectOption("Gold");
  await details.getByRole("button", { name: "Save details" }).click();
  await expect(details.getByRole("status")).toHaveText("Details saved");
  await page.keyboard.press("Escape");

  await page.getByRole("link", { name: "Search", exact: true }).click();
  await page.getByLabel("JQL").fill(`project = ${key} AND Tier = gold`);
  await page.getByRole("button", { name: "Search", exact: true }).click();
  await expect(page.getByRole("region", { name: "Results" }).getByRole("link", { name: `${key}-1` })).toBeVisible();
});
