import { expect, test } from "@playwright/test";

// Attachments: a member attaches a file to an issue, downloads it and removes it.
test("attach, download and remove a file", async ({ page }) => {
  const id = Date.now().toString(36).toUpperCase().slice(-5);
  const key = `Y${id}`.slice(0, 6);
  await page.goto("/signup");
  await page.getByLabel("Name").fill("File Keeper");
  await page.getByLabel("Email").fill(`files-${id.toLowerCase()}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await page.getByLabel("Key").fill(key);
  await page.getByLabel("Project name").fill("Files");
  await page.getByRole("button", { name: "Create project" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));

  await page.getByLabel("New issue title").fill("Crash on save");
  await page.getByRole("button", { name: "Add issue" }).click();
  await page.getByText("Crash on save").click();
  const files = page.getByRole("region", { name: "Attachments" });
  await files.getByLabel("Attach file").setInputFiles({ name: "trace.log", mimeType: "text/plain", buffer: Buffer.from("panic: boom\n") });
  const link = files.getByRole("link", { name: "trace.log" });
  await expect(link).toBeVisible();
  await expect(files).toContainText("12 B");

  const download = page.waitForEvent("download");
  await link.click();
  expect((await download).suggestedFilename()).toBe("trace.log");

  await files.getByRole("button", { name: "Remove trace.log" }).click();
  await expect(link).toHaveCount(0);
});
