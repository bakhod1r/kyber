import { expect, test } from "@playwright/test";

// B1: an admin invites someone without an account; they open the link, sign up with that address and join.
test("invite by email, sign up from the link and join the project", async ({ browser }) => {
  const id = Date.now().toString(36).slice(-6);
  const key = `I${id.toUpperCase()}`.replace(/[^A-Z0-9]/g, "").slice(0, 6);
  const guest = `guest-${id}@example.com`;

  const admin = await (await browser.newContext()).newPage();
  const signup = await admin.request.post("/api/v1/auth/signup", { data: { email: `admin-${id}@example.com`, name: "Admin", password: "a very long password" } });
  expect(signup.ok()).toBeTruthy();
  await admin.request.post("/api/v1/auth/login", { data: { email: `admin-${id}@example.com`, password: "a very long password" } });
  expect((await admin.request.post("/api/v1/projects", { data: { key, name: "Invited" } })).ok()).toBeTruthy();

  await admin.goto(`/projects/${key}/members`);
  await admin.getByLabel("Invite by email").fill(guest);
  await admin.getByRole("button", { name: "Invite" }).click();
  const link = await admin.getByRole("textbox", { name: "Invitation link" }).inputValue();
  const path = new URL(link).pathname;

  const page = await (await browser.newContext()).newPage();
  await page.goto(path);
  await expect(page.getByRole("heading", { name: "Join Invited" })).toBeVisible();
  await page.getByRole("link", { name: "Create an account" }).click();
  await page.getByLabel("Name").fill("Guest");
  await page.getByLabel("Email").fill(guest);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page).toHaveURL(new RegExp(`${path}$`));
  await page.getByRole("button", { name: "Accept invitation" }).click();
  await expect(page).toHaveURL(new RegExp(`/projects/${key}$`));
  await page.goto(`/projects/${key}/members`);
  await expect(page.getByRole("complementary").getByText("Guest")).toBeVisible();

  // The admin's pending list is empty once the invitation is accepted.
  await admin.reload();
  await expect(admin.getByText("No pending invitations.")).toBeVisible();
});
