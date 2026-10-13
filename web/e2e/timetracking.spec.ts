import { expect, test } from "@playwright/test";

// Worklog: a member estimates an issue, logs work and sees the remaining time shrink.
test("estimate and log work on an issue", async ({ page }) => {
  const id = Date.now().toString(36).toUpperCase().slice(-5);
  const key = `T${id}`.slice(0, 6);
  await page.goto("/signup");
  await page.getByLabel("Name").fill("Time Keeper");
  await page.getByLabel("Email").fill(`time-${id.toLowerCase()}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByLabel("Key").fill(key);
  await page.getByLabel("Project name").fill("Time");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));

  await page.getByLabel("New issue title").fill("Write the parser");
  await page.getByRole("button", { name: "Add issue" }).click();
  await page.getByText("Write the parser").click();
  const time = page.getByRole("region", { name: "Time tracking" });
  await time.getByLabel("Original estimate").fill("1d");
  await time.getByRole("button", { name: "Set estimate" }).click();
  await expect(time).toContainText("Estimate 1d");
  await time.getByLabel("Time spent").fill("3h");
  await time.getByLabel("Worklog comment").fill("lexer");
  await time.getByRole("button", { name: "Log work" }).click();
  await expect(time).toContainText("Logged 3h · Remaining 5h");
  await expect(time.getByRole("list", { name: "Worklogs" })).toContainText("lexer", { timeout: 10_000 }); // the list is projected from events
});
