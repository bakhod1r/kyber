import { expect, test } from "@playwright/test";

// D5: project pages in Markdown with revision history.
test("docs pages keep their history", async ({ page }) => {
  const id = Date.now().toString(36).toUpperCase().slice(-5);
  const key = `D${id}`.slice(0, 6);
  await page.goto("/signup");
  await page.getByLabel("Name").fill("Doc Writer");
  await page.getByLabel("Email").fill(`docs-${id.toLowerCase()}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByLabel("Key").fill(key);
  await page.getByLabel("Project name").fill("Docs");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));

  await page.getByRole("link", { name: "Docs", exact: true }).click();
  const docs = page.getByRole("region", { name: "Docs" });
  await docs.getByRole("button", { name: "New page" }).click();
  await docs.getByLabel("Page title").fill("Onboarding");
  await docs.getByLabel("Content (Markdown)").fill("# Welcome\n\nRead the **rules**.");
  await docs.getByRole("button", { name: "Save page" }).click();
  const article = docs.getByRole("article", { name: "Onboarding" });
  await expect(article.getByRole("heading", { name: "Welcome" })).toBeVisible();
  await expect(article.locator("strong")).toHaveText("rules");

  await article.getByRole("button", { name: "Edit" }).click();
  await docs.getByLabel("Content (Markdown)").fill("# Welcome aboard");
  await docs.getByRole("button", { name: "Save page" }).click();
  await expect(article.getByText("Version 2")).toBeVisible();
  await article.getByRole("button", { name: "History" }).click();
  await article.getByRole("button", { name: "Version 1" }).click();
  await expect(article.locator("strong")).toHaveText("rules");
});

// S86: comment on a paragraph; the 💬 count stays with it after a reload.
test("docs paragraphs take comments", async ({ page }) => {
  const id = Date.now().toString(36).toUpperCase().slice(-5);
  const key = `C${id}`.slice(0, 6);
  await page.goto("/signup");
  await page.getByLabel("Name").fill("Doc Reviewer");
  await page.getByLabel("Email").fill(`doccomments-${id.toLowerCase()}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByLabel("Key").fill(key);
  await page.getByLabel("Project name").fill("Comments");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));

  await page.getByRole("link", { name: "Docs", exact: true }).click();
  const docs = page.getByRole("region", { name: "Docs" });
  await docs.getByRole("button", { name: "New page" }).click();
  await docs.getByLabel("Page title").fill("Design");
  await docs.getByLabel("Content (Markdown)").fill("# Design\n\nWe cache sessions in memory.");
  await docs.getByRole("button", { name: "Save page" }).click();

  await docs.getByRole("button", { name: "Comment on paragraph 2" }).click();
  const panel = docs.getByRole("complementary", { name: "Comments on paragraph 2" });
  await panel.getByLabel("Comment").fill("What about several replicas?");
  await panel.getByRole("button", { name: "Comment" }).click();
  await expect(panel.getByText("What about several replicas?")).toBeVisible();

  await page.reload();
  await expect(docs.getByRole("button", { name: "1 comment on paragraph 2" })).toBeVisible();
  await docs.getByRole("button", { name: "1 comment on paragraph 2" }).click();
  await panel.getByRole("button", { name: "Remove" }).click();
  await expect(docs.getByRole("button", { name: "Comment on paragraph 2" })).toBeVisible();
});
