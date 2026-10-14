import { createHash } from "node:crypto";
import { expect, test } from "@playwright/test";

// ADR-0016: an MCP client registers, the user signs in and approves it, and the code becomes a
// token that works on the MCP endpoint.
test("an MCP client gets a token through OAuth with PKCE", async ({ page, baseURL }) => {
  const id = Date.now().toString(36).toLowerCase().slice(-6);
  const callback = new URL("/oauth-callback", baseURL).toString();
  const reg = await page.request.post("/api/v1/oauth/register", { data: { client_name: "E2E Assistant", redirect_uris: [callback] } });
  expect(reg.status()).toBe(201);
  const clientId = (await reg.json()).client_id as string;
  const verifier = "e2e-verifier-".padEnd(64, "x");
  const challenge = createHash("sha256").update(verifier).digest("base64url");
  const authorize = `/oauth/authorize?response_type=code&client_id=${clientId}&redirect_uri=${encodeURIComponent(callback)}&code_challenge=${challenge}&code_challenge_method=S256&state=st-1`;

  // Signed out: the consent screen sends the user through sign-up and back.
  await page.goto(authorize);
  await expect(page).toHaveURL(/\/login\?next=/);
  await page.getByRole("link", { name: "Create an account" }).click();
  await page.getByLabel("Name").fill("OAuth User");
  await page.getByLabel("Email").fill(`oauth-${id}@example.com`);
  await page.getByLabel("Password").fill("a very long password");
  await page.getByRole("button", { name: "Create account" }).click();
  await expect(page.getByRole("heading", { name: "Allow E2E Assistant to access Kyber?" })).toBeVisible();

  const back = page.waitForRequest((r) => r.url().startsWith(callback));
  await page.getByRole("button", { name: "Allow" }).click();
  const to = new URL((await back).url());
  expect(to.searchParams.get("state")).toBe("st-1");

  const token = await page.request.post("/api/v1/oauth/token", {
    form: { grant_type: "authorization_code", code: to.searchParams.get("code")!, client_id: clientId, redirect_uri: callback, code_verifier: verifier },
  });
  expect(token.status()).toBe(200);
  const accessToken = (await token.json()).access_token as string;
  const mcp = await page.request.post("/api/v1/mcp", {
    headers: { Authorization: `Bearer ${accessToken}` },
    data: { jsonrpc: "2.0", id: 1, method: "tools/list" },
  });
  expect(mcp.status()).toBe(200);
  expect(JSON.stringify(await mcp.json())).toContain("search_issues");
});
