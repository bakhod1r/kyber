import { expect, test } from "@playwright/test";

// D6: an intake form creates an issue from its answers.
test("a form submission becomes an issue", async ({ page }) => {
  const id = Date.now().toString(36).toUpperCase().slice(-5);
  const key = `F${id}`.slice(0, 6);
  await page.goto("/signup");
  await page.getByLabel("Name").fill("Form Builder");
  await page.getByLabel("Email").fill(`forms-${id.toLowerCase()}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByLabel("Key").fill(key);
  await page.getByLabel("Project name").fill("Forms");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));

  await page.getByRole("link", { name: "Forms", exact: true }).click();
  const forms = page.getByRole("region", { name: "Forms" });
  await forms.getByRole("button", { name: "New form" }).click();
  await forms.getByLabel("Form name").fill("Bug report");
  await forms.getByLabel("Creates").selectOption("bug");
  await forms.getByRole("button", { name: "Add question" }).click();
  await forms.getByLabel("Question 1").fill("Steps");
  await forms.getByLabel("Required").check();
  await forms.getByRole("button", { name: "Save form" }).click();

  await forms.getByRole("button", { name: "Fill in Bug report" }).click();
  await forms.getByLabel("Summary").fill("Checkout crashes");
  await forms.getByLabel("Steps *").fill("Pay with card");
  await forms.getByRole("button", { name: "Submit" }).click();
  await forms.getByRole("status").getByRole("button", { name: `${key}-1` }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("textbox", { name: "Title", exact: true })).toHaveValue("Checkout crashes");
  await expect(dialog.getByRole("textbox", { name: "Description", exact: true })).toHaveValue(/\*\*Steps\*\*\s+Pay with card/);
});
