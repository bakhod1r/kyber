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

  // KYB-S17: open the issue, edit details, assign, comment — all persisted.
  await page.getByRole("button", { name: "Ship the board" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByLabel("Description").fill("Drag cards between columns.\nKeyboard support later.");
  await dialog.getByLabel("Priority").selectOption("highest");
  await dialog.getByLabel("Assignee").selectOption({ label: "E2E User" });
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(dialog.getByText("Saved")).toBeVisible();
  await dialog.getByLabel("Add a comment").fill("Looks great!");
  await dialog.getByRole("button", { name: "Comment" }).click();
  await expect(dialog.getByText("Looks great!")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.getByLabel("Priority: highest")).toBeVisible();
  await expect(page.getByTitle("E2E User")).toHaveText("EU");
  await page.reload();
  await page.getByRole("button", { name: "Ship the board" }).click();
  await expect(page.getByRole("dialog").getByLabel("Description")).toHaveValue("Drag cards between columns.\nKeyboard support later.");
  await expect(page.getByRole("dialog").getByText("Looks great!")).toBeVisible();
  await page.keyboard.press("Escape");

  // KYB-S18/19/20: plan a sprint in the backlog, start it, complete it.
  await page.getByLabel("New issue title").fill("Write release notes");
  await page.getByRole("button", { name: "Add issue" }).click();
  await expect(page.getByRole("region", { name: "To Do" }).getByText("Write release notes")).toBeVisible();
  await page.getByRole("link", { name: "Backlog" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}/backlog$`));
  await page.getByLabel("Sprint name").fill("Sprint 1");
  await page.getByLabel("Goal").fill("Ship v1");
  await page.getByRole("button", { name: "Create sprint" }).click();
  const sprint1 = page.getByRole("region", { name: "Sprint 1" });
  await expect(sprint1).toBeVisible();
  const backlog = page.getByRole("region", { name: "Backlog" });
  // Rank: move "Write release notes" above "Ship the board" inside the backlog.
  await backlog.getByRole("listitem").filter({ hasText: "Write release notes" }).dragTo(backlog.getByRole("listitem").filter({ hasText: "Ship the board" }));
  await expect(backlog.getByRole("listitem").first()).toContainText("Write release notes");
  // Plan both issues into the sprint.
  await backlog.getByRole("listitem").filter({ hasText: "Ship the board" }).dragTo(sprint1);
  await expect(sprint1.getByText("Ship the board")).toBeVisible();
  await backlog.getByRole("listitem").filter({ hasText: "Write release notes" }).dragTo(sprint1);
  await expect(sprint1.getByText("Write release notes")).toBeVisible();
  await expect(backlog.getByText("Drag issues here")).toBeVisible();
  await sprint1.getByRole("button", { name: "Start sprint" }).click();
  await expect(sprint1.getByText("active")).toBeVisible();
  // The board now shows the active sprint.
  await page.getByRole("link", { name: "Board" }).click();
  await expect(page.getByText("Sprint 1")).toBeVisible();
  await expect(page.getByText("— Ship v1")).toBeVisible();
  const ip = page.getByRole("region", { name: "In Progress" });
  await ip.getByText("Ship the board").dragTo(page.getByRole("region", { name: "Done" }));
  await expect(page.getByRole("region", { name: "Done" }).getByText("Ship the board")).toBeVisible();
  // Complete: the unfinished issue returns to the backlog.
  await page.getByRole("link", { name: "Backlog" }).click();
  await page.getByRole("region", { name: "Sprint 1" }).getByRole("button", { name: "Complete sprint" }).click();
  await expect(page.getByRole("status")).toContainText("Sprint 1 completed: 1 done, 1 returned to the backlog.");
  await expect(page.getByRole("region", { name: "Sprint 1" })).toHaveCount(0);
  await expect(page.getByRole("region", { name: "Backlog" }).getByText("Write release notes")).toBeVisible();
  await page.getByRole("link", { name: "Board" }).click();

  // Inviting an unknown email surfaces the server error.
  await page.getByLabel("Invite by email").fill("nobody@example.com");
  await page.getByRole("button", { name: "Invite" }).click();
  await expect(page.getByRole("alert")).toContainText("no user with that email");

  await page.getByRole("button", { name: "Log out" }).click();
  await expect(page.getByRole("heading", { name: "Log in to Kyber" })).toBeVisible();
  await page.goto(`/projects/${key}`);
  await expect(page.getByRole("heading", { name: "Log in to Kyber" })).toBeVisible();
});
