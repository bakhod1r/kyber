import { expect, test } from "@playwright/test";

// KYB-S43: an admin adds a "Code Review" status; it becomes a board column issues can move into.
test("a project admin customises the workflow and the board follows", async ({ page }) => {
  const id = Date.now().toString(36).toUpperCase().slice(-5);
  const key = `W${id}`.slice(0, 6);

  await page.goto("/signup");
  await page.getByLabel("Name").fill("Workflow Admin");
  await page.getByLabel("Email").fill(`wf-${id.toLowerCase()}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByLabel("Key").fill(key);
  await page.getByLabel("Project name").fill("Workflow");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));

  await page.getByRole("link", { name: "Settings", exact: true }).click();

  await page.getByRole("link", { name: "Workflow", exact: true }).click();
  await page.getByLabel("New status name").fill("Code Review");
  await page.getByLabel("New status category").selectOption("in_progress");
  await page.getByRole("button", { name: "Add status" }).click();
  await page.getByRole("button", { name: "Save workflow" }).click();
  await expect(page.getByRole("status")).toHaveText("Workflow saved");

  await page.getByRole("link", { name: "Active sprints", exact: true }).click();
  await page.getByLabel("New issue title").fill("Review me");
  await page.getByRole("button", { name: "Add issue" }).click();
  const review = page.getByRole("region", { name: "Code Review" });
  await expect(review).toBeVisible();
  await page.getByRole("region", { name: "To Do" }).getByText("Review me").dragTo(review);
  await expect(review.getByText("Review me")).toBeVisible();
  await page.reload();
  await expect(page.getByRole("region", { name: "Code Review" }).getByText("Review me")).toBeVisible();
});
