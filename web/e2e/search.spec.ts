import { expect, test } from "@playwright/test";

// JQL search: find an issue, save the search as a filter and run it again.
test("search with JQL and save a filter", async ({ page }) => {
  const id = Date.now().toString(36).toUpperCase().slice(-5);
  const key = `S${id}`.slice(0, 6);
  await page.goto("/signup");
  await page.getByLabel("Name").fill("Searcher");
  await page.getByLabel("Email").fill(`search-${id.toLowerCase()}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByLabel("Key").fill(key);
  await page.getByLabel("Project name").fill("Search");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));
  await page.getByLabel("New issue title").fill("Login crashes");
  await page.getByLabel("Type").selectOption("bug");
  await page.getByRole("button", { name: "Add issue" }).click();
  await expect(page.getByText("Login crashes")).toBeVisible(); // the form resets after the first add
  await page.getByLabel("New issue title").fill("Write docs");
  await page.getByLabel("Type").selectOption("task"); // the form keeps the last type
  await page.getByRole("button", { name: "Add issue" }).click();
  await expect(page.getByText("Write docs")).toBeVisible();

  await page.getByRole("link", { name: "Search", exact: true }).click();
  await page.getByLabel("JQL").fill(`project = ${key} AND type = bug`);
  await page.getByRole("button", { name: "Search", exact: true }).click();
  const results = page.getByRole("region", { name: "Results" });
  await expect(results.getByRole("link", { name: `${key}-1` })).toBeVisible();
  await expect(results).not.toContainText("Write docs");
  await expect(results).toContainText("1 issue");

  await page.getByLabel("Save this search as").fill("My bugs");
  await page.getByRole("button", { name: "Save filter" }).click();
  await expect(page.getByRole("status")).toHaveText("Filter saved");
  await page.getByLabel("JQL").fill(`project = ${key}`);
  await page.getByRole("button", { name: "Search", exact: true }).click();
  await expect(results).toContainText("2 issues");
  await page.getByRole("complementary", { name: "Saved filters" }).getByRole("button", { name: "My bugs", exact: true }).click();
  await expect(results).toContainText("1 issue");
});
