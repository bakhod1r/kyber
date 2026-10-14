import { expect, test } from "@playwright/test";

// Labels: tag an issue, filter the board by label, and find it with an autocompleted JQL query.
test("label an issue, filter the board and search by label", async ({ page }) => {
  const id = Date.now().toString(36).toUpperCase().slice(-5);
  const key = `X${id}`.slice(0, 6);
  await page.goto("/signup");
  await page.getByLabel("Name").fill("Label Maker");
  await page.getByLabel("Email").fill(`labels-${id.toLowerCase()}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByLabel("Key").fill(key);
  await page.getByLabel("Project name").fill("Labels");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));

  for (const title of ["Restyle login", "Tune queries"]) {
    await page.getByLabel("New issue title").fill(title);
    await page.getByRole("button", { name: "Add issue" }).click();
    await expect(page.getByText(title)).toBeVisible();
  }
  await page.getByText("Restyle login").click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Labels").fill("ui frontend");
  await dialog.getByRole("button", { name: "Save", exact: true }).click();
  await expect(dialog.getByText("Saved")).toBeVisible();
  await page.keyboard.press("Escape");

  await page.locator("summary[aria-label='Filter by Label']").click();
  await page.getByRole("group", { name: "Filter by Label" }).getByRole("checkbox", { name: "ui" }).check();
  await expect(page.getByText("Restyle login")).toBeVisible();
  await expect(page.getByText("Tune queries")).toBeHidden();

  await page.goto("/search");
  const jql = page.getByRole("combobox", { name: "JQL" });
  await jql.fill(`project = ${key} AND lab`);
  await page.getByRole("option", { name: "labels" }).click();
  await jql.pressSequentially("= ");
  await page.getByRole("option", { name: "frontend" }).click();
  await jql.press("Escape");
  await page.getByRole("button", { name: "Search", exact: true }).click();
  const results = page.getByRole("region", { name: "Results" });
  await expect(results).toContainText("Restyle login");
  await expect(results).not.toContainText("Tune queries");
});
