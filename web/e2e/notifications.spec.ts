import { type Browser, type Page, expect, test } from "@playwright/test";

async function signUp(browser: Browser, name: string, email: string): Promise<Page> {
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  await page.goto("/signup");
  await page.getByLabel("Name").fill(name);
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByRole("heading", { name: "Projects" })).toBeVisible();
  return page;
}

// KYB-S23/S24: assignment and @mention reach the other user's bell; clicking opens the issue.
test("assignment and mention notify a teammate", async ({ browser }) => {
  const id = Date.now().toString(36).slice(-5);
  const key = `N${id.toUpperCase()}`.slice(0, 6);
  const alice = await signUp(browser, "Alice Ali", `alice-${id}@example.com`);
  const bob = await signUp(browser, "Bob Bek", `bob-${id}@example.com`);

  await alice.getByLabel("Key").fill(key);
  await alice.getByLabel("Project name").fill("Notify test");
  await alice.getByRole("button", { name: "Create project" }).click();
  await alice.getByLabel("Invite by email").fill(`bob-${id}@example.com`);
  await alice.getByRole("button", { name: "Invite" }).click();
  await expect(alice.locator(".members").getByText("Bob Bek")).toBeVisible();
  await alice.getByLabel("New issue title").fill("Fix login");
  await alice.getByRole("button", { name: "Add issue" }).click();

  await alice.getByRole("button", { name: "Fix login" }).click();
  const dialog = alice.getByRole("dialog");
  await expect(dialog.getByText("Reporter: Alice Ali")).toBeVisible();
  await dialog.getByLabel("Assignee").selectOption({ label: "Bob Bek" });
  await dialog.getByRole("button", { name: "Save" }).click();
  await expect(dialog.getByText("Saved")).toBeVisible();
  await dialog.getByLabel("Add a comment").fill(`@bob-${id}@example.com can you take this today?`);
  await dialog.getByRole("button", { name: "Comment" }).click();
  await expect(dialog.getByText("can you take this today?")).toBeVisible();

  // Bob's bell picks both up (poll by reloading; the UI also polls every 30 s).
  await expect(async () => {
    await bob.reload();
    await expect(bob.getByRole("button", { name: "Notifications (2 unread)" })).toBeVisible({ timeout: 1000 });
  }).toPass({ timeout: 15_000 });
  await bob.getByRole("button", { name: "Notifications (2 unread)" }).click();
  const panel = bob.getByRole("dialog", { name: "Notifications" });
  await expect(panel.getByRole("button", { name: new RegExp(`Alice Ali assigned you ${key}-1`) })).toBeVisible();
  await panel.getByRole("button", { name: new RegExp(`Alice Ali mentioned you on ${key}-1`) }).click();
  await expect(bob).toHaveURL(new RegExp(`/projects/${key}\\?issue=${key}-1$`));
  await expect(bob.getByRole("dialog", { name: new RegExp(`${key}-1`) })).toBeVisible();
  await expect(bob.getByRole("button", { name: "Notifications (1 unread)" })).toBeVisible();

  // Alice (the actor) got nothing.
  await alice.reload();
  await expect(alice.getByRole("button", { name: "Notifications" })).toBeVisible();
});
