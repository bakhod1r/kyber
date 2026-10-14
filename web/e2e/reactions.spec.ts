import { expect, test } from "@playwright/test";

// Comment reactions (F4): a member reacts to a comment with an emoji and takes it back.
test("react to a comment and take the reaction back", async ({ page }) => {
  const id = Date.now().toString(36).toUpperCase().slice(-5);
  const key = `R${id}`.slice(0, 6);
  await page.goto("/signup");
  await page.getByLabel("Name").fill("Emoji Fan");
  await page.getByLabel("Email").fill(`react-${id.toLowerCase()}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByLabel("Key").fill(key);
  await page.getByLabel("Project name").fill("Reactions");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));

  await page.getByLabel("New issue title").fill("Ship reactions");
  await page.getByRole("button", { name: "Add issue" }).click();
  await page.getByText("Ship reactions").click();
  const comments = page.getByRole("region", { name: "Comments" });
  await comments.getByLabel("Add a comment").fill("Looks great");
  await comments.getByRole("button", { name: "Comment" }).click();
  await expect(comments.getByText("Looks great")).toBeVisible();

  await comments.getByRole("button", { name: "Add reaction" }).click();
  await comments.getByRole("button", { name: "React with 🎉" }).click();
  const chip = comments.getByRole("button", { name: "🎉 reacted by 1" });
  await expect(chip).toHaveAttribute("aria-pressed", "true");

  await chip.click();
  await expect(comments.getByRole("button", { name: /^🎉 reacted by/ })).toHaveCount(0);
});
