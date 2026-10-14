import { type APIRequestContext, expect, test } from "@playwright/test";

// S64: Activity lists my mentions and unread conversations; opening the conversation reads them.
test("activity shows mentions and unread conversations", async ({ browser }) => {
  const id = Date.now().toString(36).slice(-5);
  const key = `A${id.toUpperCase()}`.slice(0, 6);
  const signIn = async (who: string) => {
    const ctx = await browser.newContext();
    const call = (r: APIRequestContext, method: string, path: string, data?: unknown) =>
      r.fetch(path, { method, data: data ?? {}, headers: { "Content-Type": "application/json" } }).then((x) => (x.status() === 204 ? null : x.json()));
    const email = `${who}-${id}@example.com`;
    await call(ctx.request, "POST", "/api/v1/auth/signup", { email, name: who, password: "a very long password" });
    await call(ctx.request, "POST", "/api/v1/auth/login", { email, password: "a very long password" });
    return { ctx, email, call: (m: string, p: string, d?: unknown) => call(ctx.request, m, p, d) };
  };
  const ali = await signIn("Ali");
  const bobur = await signIn("Bobur");
  await ali.call("POST", "/api/v1/projects", { key, name: "Activity" });
  await ali.call("POST", `/api/v1/projects/${key}/members`, { email: bobur.email, role: "member" });
  const channel = await ali.call("POST", "/api/v1/channels", { name: `act-${id}` });
  await bobur.call("POST", `/api/v1/conversations/${channel.id}/join`);
  await ali.call("POST", `/api/v1/conversations/${channel.id}/messages`, { body: `@${bobur.email} can you check the deploy?` });

  await expect.poll(async () => (await bobur.call("GET", "/api/v1/notifications")).unread).toBe(1); // delivered by the relay
  const b = await bobur.ctx.newPage();
  await b.goto("/chat");
  const mentions = b.getByRole("list", { name: "Mentions" });
  await expect(mentions).toContainText("can you check the deploy?");
  await expect(b.getByRole("list", { name: "Unread conversations" })).toContainText(`# act-${id}`);

  await mentions.getByRole("link", { name: `Ali in #act-${id}` }).click();
  await expect(b.getByRole("log", { name: "Messages" })).toContainText("can you check the deploy?");
  await b.getByRole("link", { name: "🔔 Activity" }).click();
  await expect(b.getByText("You are all caught up.")).toBeVisible();
  await expect.poll(async () => (await bobur.call("GET", "/api/v1/notifications")).unread).toBe(0);

  // S65: save it for later with a reminder; it shows under Saved, and can be removed.
  await b.goto(`/chat/${channel.id}`);
  await b.getByLabel("Save for later").selectOption({ label: "Remind me: in 1 hour" });
  await expect(b.getByRole("button", { name: "🔖 Saved" })).toBeVisible();
  await b.getByRole("link", { name: "🔔 Activity" }).click();
  const saved = b.getByRole("list", { name: "Saved messages" });
  await expect(saved).toContainText("can you check the deploy?");
  await expect(saved).toContainText("⏰");
  await saved.getByRole("button", { name: "Remove" }).click();
  await expect(b.getByText("Nothing saved.")).toBeVisible();

  // S66: Bobur sets a status with Do Not Disturb; Ali sees it next to his name.
  await b.getByRole("button", { name: "😀 Set a status" }).click();
  await b.getByRole("button", { name: "🏖" }).click();
  await b.getByLabel("Do not disturb").selectOption("60");
  await b.getByRole("form", { name: "Status" }).getByRole("button", { name: "Save" }).click();
  await expect(b.getByRole("button", { name: /On vacation · 🌙 Do not disturb/ })).toBeVisible();
  await bobur.call("POST", `/api/v1/conversations/${channel.id}/messages`, { body: "back on Monday" });
  const a = await ali.ctx.newPage();
  await a.goto(`/chat/${channel.id}`);
  const last = a.getByRole("article").filter({ hasText: "back on Monday" });
  await expect(last.getByTitle("On vacation")).toHaveText("🏖");
  await expect(last.getByLabel("Do not disturb")).toBeVisible();

  // S67: Ali schedules a message (only he sees it until then) and sets a /remind.
  const box = a.getByRole("textbox", { name: `Message # act-${id}` });
  await box.fill("release notes are ready");
  await a.getByLabel("Send later").selectOption({ label: "Send tomorrow at 9:00" });
  await expect(a.getByText(/🕒 Scheduled for/)).toBeVisible();
  await expect(a.getByRole("log", { name: "Messages" })).not.toContainText("release notes are ready");
  await box.fill("/remind me in 20 minutes to check the dashboards");
  await box.press("Enter");
  await expect(a.getByText(/⏰ I'll remind you at .*check the dashboards/)).toBeVisible();
  await box.fill("/remind me someday");
  await box.press("Enter");
  await expect(a.getByText(/Try: \/remind me in 20 minutes/)).toBeVisible();
  await a.getByRole("link", { name: "🔔 Activity" }).click();
  const scheduled = a.getByRole("list", { name: "Scheduled messages" });
  await expect(scheduled).toContainText("release notes are ready");
  await scheduled.getByRole("button", { name: "Cancel" }).click();
  await expect(scheduled).toHaveCount(0);

  // S70: Ali makes @reviewers with Bobur; a message to @reviewers reaches Bobur.
  await a.getByRole("button", { name: "New group" }).click();
  const form = a.getByRole("form", { name: "Group" });
  await form.getByLabel("Handle").fill(`rev-${id}`);
  await form.getByLabel("Bobur").check();
  await form.getByRole("button", { name: "Save group" }).click();
  await expect(a.getByRole("list", { name: "User groups" })).toContainText(`@rev-${id}`);
  const before = (await bobur.call("GET", "/api/v1/notifications")).items.length;
  await ali.call("POST", `/api/v1/conversations/${channel.id}/messages`, { body: `@rev-${id} take a look` });
  await expect.poll(async () => (await bobur.call("GET", "/api/v1/notifications")).items.length).toBe(before + 1);
  await a.goto(`/chat/${channel.id}`);
  await expect(a.getByRole("log", { name: "Messages" }).locator(".chat-mention", { hasText: `@rev-${id}` })).toBeVisible();

  // S71: Bobur moves the channel to his own "Later" section and mutes it.
  await b.goto(`/chat/${channel.id}`);
  const opts = b.getByLabel(`Options for # act-${id}`);
  await opts.selectOption({ label: "New section…" });
  await b.getByLabel("Section name").fill("Later");
  await b.getByLabel("Section name").press("Enter");
  const later = b.getByRole("region", { name: "Later" });
  await expect(later).toContainText(`# act-${id}`);
  await b.getByLabel(`Options for # act-${id}`).selectOption({ label: "🔕 Mute" });
  await expect(later.getByLabel("Muted")).toBeVisible();
  const n = (await bobur.call("GET", "/api/v1/notifications")).items.length;
  await ali.call("POST", `/api/v1/conversations/${channel.id}/messages`, { body: `@${bobur.email} are you there?` });
  await b.waitForTimeout(1500);
  expect((await bobur.call("GET", "/api/v1/notifications")).items.length).toBe(n); // muted

  // S80: Ali adds a welcome workflow; when Malika joins, the channel greets her.
  await a.goto(`/chat/${channel.id}`);
  await a.getByRole("button", { name: "⚙ Settings" }).click();
  await a.getByLabel("Workflow message").fill("Welcome {name}! Start with the pinned notes.");
  await a.getByRole("button", { name: "Add workflow" }).click();
  await expect(a.getByRole("group", { name: "Workflows" })).toContainText("When someone joins");
  const malika = await signIn("Malika");
  await malika.call("POST", `/api/v1/conversations/${channel.id}/join`);
  await expect(a.getByRole("article", { name: /Workflow/ })).toContainText("Welcome Malika! Start with the pinned notes.");

  // S81: Ali adds an incoming webhook; CI posts through it without signing in.
  await a.getByLabel("Webhook name").fill("CI");
  await a.getByRole("button", { name: "Add webhook" }).click();
  const hookUrl = await a.getByLabel("Webhook URL value").inputValue();
  expect(hookUrl).toContain("/api/v1/hooks/");
  const res = await fetch(hookUrl, { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ text: "build #42 passed" }) });
  expect(res.status).toBe(204);
  await expect(a.getByRole("article", { name: /Webhook/ })).toContainText("CI: build #42 passed");

  // S85: Bobur drags his "Later" section above "Alpha"; the order stays after a reload.
  const second = await ali.call("POST", "/api/v1/channels", { name: `alpha-${id}` });
  await bobur.call("POST", `/api/v1/conversations/${second.id}/join`);
  await bobur.call("PUT", `/api/v1/conversations/${second.id}/prefs`, { level: "mentions", section: "Alpha" });
  await b.goto("/chat");
  const titles = () => b.getByRole("navigation", { name: "Conversations" }).locator("section h2").allTextContents();
  await expect.poll(async () => (await titles()).slice(0, 2).map((t) => t.replace("↑", "").trim())).toEqual(["Alpha", "Later"]);
  await b.getByRole("heading", { name: /^Later/ }).dragTo(b.getByRole("heading", { name: /^Alpha/ }));
  await expect.poll(async () => (await titles()).slice(0, 2).map((t) => t.replace("↑", "").trim())).toEqual(["Later", "Alpha"]);
  await b.reload();
  await expect.poll(async () => (await titles()).slice(0, 2).map((t) => t.replace("↑", "").trim())).toEqual(["Later", "Alpha"]);
});
