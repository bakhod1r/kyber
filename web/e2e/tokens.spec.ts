import { expect, test } from "@playwright/test";

// Access tokens: create a token in settings, use it as a Bearer credential, revoke it.
test("create, use and revoke a personal access token", async ({ page, request }) => {
  const id = Date.now().toString(36);
  const email = `tokens-${id}@example.com`;
  await page.goto("/signup");
  await page.getByLabel("Name").fill("Token Owner");
  await page.getByLabel("Email").fill(email);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await page.waitForURL((u) => !u.pathname.startsWith("/signup"), { timeout: 15_000 });

  await page.goto("/settings");
  const tokens = page.getByRole("region", { name: "Access tokens" });
  await tokens.getByLabel("Token name").fill("Claude Code");
  await tokens.getByRole("button", { name: "Create token" }).click();
  const secret = (await tokens.getByLabel("New token").textContent()) ?? "";
  expect(secret).toMatch(/^kpat_/);

  const auth = { Authorization: `Bearer ${secret}` };
  const me = await request.get("/api/v1/me", { headers: auth });
  expect(me.status()).toBe(200);
  expect((await me.json()).email).toBe(email);

  await tokens.getByRole("button", { name: "Revoke Claude Code" }).click();
  await expect(tokens.getByRole("button", { name: "Revoke Claude Code" })).toHaveCount(0);
  expect((await request.get("/api/v1/me", { headers: auth })).status()).toBe(401);
});
