import { expect, test } from "@playwright/test";

// D4: a goal measures its progress by JQL over the project's issues.
test("goals track progress by JQL", async ({ page }) => {
  const id = Date.now().toString(36).toUpperCase().slice(-5);
  const key = `G${id}`.slice(0, 6);
  await page.goto("/signup");
  await page.getByLabel("Name").fill("Goal Setter");
  await page.getByLabel("Email").fill(`goals-${id.toLowerCase()}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByLabel("Key").fill(key);
  await page.getByLabel("Project name").fill("Goals");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));
  for (const title of ["First bug", "Second bug"]) {
    await page.getByLabel("New issue title").fill(title);
    await page.getByLabel("Type").selectOption("bug");
    await page.getByRole("button", { name: "Add issue" }).click();
    await expect(page.getByText(title)).toBeVisible();
  }

  await page.getByRole("link", { name: "Goals", exact: true }).click();
  const goals = page.getByRole("region", { name: "Goals" });
  await goals.getByLabel("Goal name").fill("Zero bugs");
  await goals.getByLabel("Issues (JQL)").fill("type =");
  await goals.getByRole("button", { name: "Add goal" }).click();
  await expect(goals.getByRole("alert")).toBeVisible();
  await goals.getByLabel("Issues (JQL)").fill("type = bug");
  await goals.getByRole("button", { name: "Add goal" }).click();
  await expect(goals.getByText("0 of 2 issues done (0%)")).toBeVisible();
  await goals.getByLabel("Status of Zero bugs").selectOption("at_risk");
  await page.reload();
  await expect(page.getByLabel("Status of Zero bugs")).toHaveValue("at_risk");
});
