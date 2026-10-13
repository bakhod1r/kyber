import { expect, test } from "@playwright/test";

// E1: a project admin publishes an announcement to the project.
test("project announcements", async ({ page }) => {
  const id = Date.now().toString(36).toUpperCase().slice(-5);
  const key = `A${id}`.slice(0, 6);
  await page.goto("/signup");
  await page.getByLabel("Name").fill("Team Lead");
  await page.getByLabel("Email").fill(`announce-${id.toLowerCase()}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByRole("region", { name: "Workspace announcements" })).toBeVisible();
  await page.getByLabel("Key").fill(key);
  await page.getByLabel("Project name").fill("Announce");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));

  await page.getByRole("link", { name: "Announcements", exact: true }).click();
  const feed = page.getByRole("region", { name: "Announcements" });
  await feed.getByRole("button", { name: "New announcement" }).click();
  await feed.getByLabel("Headline").fill("Release on Friday");
  await feed.getByLabel("Message (Markdown)").fill("Deploy at **16:00**, freeze from noon.");
  await feed.getByRole("button", { name: "Publish to everyone" }).click();
  const post = feed.getByRole("article", { name: "Release on Friday" });
  await expect(post).toBeVisible();
  await expect(post.locator("strong")).toHaveText("16:00");
});
