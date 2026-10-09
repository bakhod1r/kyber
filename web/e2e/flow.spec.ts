import { expect, test } from "@playwright/test";

// KYB-S14 end-to-end: sign up → project → issue → drag across the board → invite → log out.
test("a new user runs a project on the Kanban board", async ({ page }) => {
  const id = Date.now().toString(36).toUpperCase().slice(-5);
  const email = `e2e-${id.toLowerCase()}@example.com`;
  const key = `E${id}`.slice(0, 6);

  await page.goto("/");
  await expect(page.getByRole("heading", { name: "Log in to Kyber" })).toBeVisible();
  await page.getByRole("link", { name: "Create an account" }).click();
  await page.getByLabel("Name").fill("E2E User");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();

  await expect(page.getByText("No projects yet")).toBeVisible();
  await page.getByLabel("Key").fill(key);
  await page.getByLabel("Project name").fill("End to end");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));

  await page.getByLabel("New issue title").fill("Ship the board");
  await page.getByRole("button", { name: "Add issue" }).click();
  const todo = page.getByRole("region", { name: "To Do" });
  const inProgress = page.getByRole("region", { name: "In Progress" });
  const done = page.getByRole("region", { name: "Done" });
  await expect(todo.getByText("Ship the board")).toBeVisible();

  // Disallowed move (todo → done) is rejected and the card returns.
  await todo.getByText("Ship the board").dragTo(done);
  await expect(page.getByRole("alert")).toContainText("transition not allowed");
  await expect(todo.getByText("Ship the board")).toBeVisible();

  // Allowed move persists across a reload.
  await todo.getByText("Ship the board").dragTo(inProgress);
  await expect(inProgress.getByText("Ship the board")).toBeVisible();
  await page.reload();
  await expect(page.getByRole("region", { name: "In Progress" }).getByText("Ship the board")).toBeVisible();

  // Inviting an unknown email surfaces the server error.
  await page.getByLabel("Invite by email").fill("nobody@example.com");
  await page.getByRole("button", { name: "Invite" }).click();
  await expect(page.getByRole("alert")).toContainText("no user with that email");

  await page.getByRole("button", { name: "Log out" }).click();
  await expect(page.getByRole("heading", { name: "Log in to Kyber" })).toBeVisible();
  await page.goto(`/projects/${key}`);
  await expect(page.getByRole("heading", { name: "Log in to Kyber" })).toBeVisible();
});
