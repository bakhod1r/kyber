import { expect, test } from "@playwright/test";

// D3: start and due dates on an issue put it on the project's timeline.
test("timeline shows scheduled issues", async ({ page }) => {
  const id = Date.now().toString(36).toUpperCase().slice(-5);
  const key = `T${id}`.slice(0, 6);
  await page.goto("/signup");
  await page.getByLabel("Name").fill("Planner Of Things");
  await page.getByLabel("Email").fill(`timeline-${id.toLowerCase()}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByLabel("Key").fill(key);
  await page.getByLabel("Project name").fill("Timeline");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));
  await page.getByLabel("New issue title").fill("Ship payments");
  await page.getByLabel("Type").selectOption("task");
  await page.getByRole("button", { name: "Add issue" }).click();
  await page.getByText("Ship payments").click();

  const details = page.getByRole("region", { name: "Details" });
  await details.getByLabel("Due date").fill("2026-10-12");
  await details.getByLabel("Start date").fill("2026-10-20");
  await details.getByRole("button", { name: "Save details" }).click();
  await expect(details.getByRole("alert")).toBeVisible();
  await details.getByLabel("Start date").fill("2026-10-05");
  await details.getByRole("button", { name: "Save details" }).click();
  await expect(details.getByRole("status")).toHaveText("Details saved");
  await page.keyboard.press("Escape");

  await page.getByRole("link", { name: "Timeline", exact: true }).click();
  await expect(page.getByRole("img", { name: `${key}-1: 2026-10-05 → 2026-10-12` })).toBeVisible();
});
