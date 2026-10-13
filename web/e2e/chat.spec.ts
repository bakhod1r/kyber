import { type APIRequestContext, expect, test } from "@playwright/test";

// S50–S53: two people chat in a channel; messages, typing and reactions arrive live over the stream.
test("chat between two people arrives live", async ({ browser }) => {
  const id = Date.now().toString(36).slice(-5);
  const key = `C${id.toUpperCase()}`.slice(0, 6);
  const signIn = async (who: string) => {
    const ctx = await browser.newContext();
    const call = (r: APIRequestContext, method: string, path: string, data?: unknown) =>
      r.fetch(path, { method, data, headers: { "Content-Type": "application/json" } }).then((x) => (x.status() === 204 ? null : x.json()));
    const email = `${who}-${id}@example.com`;
    await call(ctx.request, "POST", "/api/v1/auth/signup", { email, name: who, password: "a very long password" });
    await call(ctx.request, "POST", "/api/v1/auth/login", { email, password: "a very long password" });
    return { ctx, email, call: (m: string, p: string, d?: unknown) => call(ctx.request, m, p, d) };
  };
  const ali = await signIn("Ali");
  const bobur = await signIn("Bobur");
  await ali.call("POST", "/api/v1/projects", { key, name: "Chat" });
  await ali.call("POST", `/api/v1/projects/${key}/members`, { email: bobur.email, role: "member" });
  await ali.call("POST", `/api/v1/projects/${key}/issues`, { title: "Login page", type: "task" });
  const channel = await ali.call("POST", "/api/v1/channels", { name: `team-${id}` });
  await bobur.call("POST", `/api/v1/conversations/${channel.id}/join`);

  const a = await ali.ctx.newPage();
  const b = await bobur.ctx.newPage();
  await a.goto(`/chat/${channel.id}`);
  await b.goto(`/chat/${channel.id}`);
  await expect(b.getByRole("heading", { name: `# team-${id}` })).toBeVisible();

  const box = a.getByRole("textbox", { name: `Message # team-${id}` });
  await box.fill(`${key}-1 is ready, @${bobur.email}`);
  await box.press("Enter");
  const log = b.getByRole("log", { name: "Messages" });
  await expect(log).toContainText(`${key}-1 is ready`); // live, without reloading
  await b.getByRole("button", { name: "React 🎉" }).click();
  await expect(a.getByRole("button", { name: "🎉 1" })).toBeVisible();

  await b.getByRole("textbox", { name: `Message # team-${id}` }).pressSequentially("hi");
  await expect(a.getByText("Bobur is typing…")).toBeVisible();

  await b.goto(`/chat/${channel.id}`); // after a reload the REST history carries the issue preview
  await expect(b.getByRole("link", { name: `${key}-1 Login page` })).toBeVisible();
  await expect.poll(async () => ((await bobur.call("GET", "/api/v1/notifications")).items as unknown[]).length).toBe(1);

  // A picture from Ali shows up for Bobur, served only to members.
  const png = Buffer.from("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==", "base64");
  await a.getByLabel("Attach a file").setInputFiles({ name: "dot.png", mimeType: "image/png", buffer: png });
  const img = b.getByRole("img", { name: "dot.png" });
  await expect(img).toBeVisible();
  await expect.poll(() => img.evaluate((el: HTMLImageElement) => el.naturalWidth)).toBe(1);

  // S68: in a DM, "Seen" appears under my message once the other has read it.
  const aliId = (await ali.call("GET", "/api/v1/me")).id;
  const dm = await bobur.call("POST", "/api/v1/dms", { user_ids: [aliId] });
  await b.goto(`/chat/${dm.id}`);
  const dmBox = b.getByRole("textbox", { name: /^Message / });
  await dmBox.fill("did you see the build?");
  await dmBox.press("Enter");
  await expect(b.getByRole("log", { name: "Messages" })).toContainText("did you see the build?");
  await expect(b.getByLabel("Seen")).toHaveCount(0);
  await a.goto(`/chat/${dm.id}`);
  await expect(a.getByRole("log", { name: "Messages" })).toContainText("did you see the build?");
  await expect(b.getByLabel("Seen")).toBeVisible(); // live, without reloading

  // S72: slash commands create, assign and move issues; only Ali sees the answers.
  await a.goto(`/chat/${channel.id}`);
  const cbox = a.getByRole("textbox", { name: `Message # team-${id}` });
  await cbox.fill(`/issue ${key} Fix the flaky login test`);
  await cbox.press("Enter");
  await expect(a.getByText(`✅ Created ${key}-2: Fix the flaky login test`)).toBeVisible();
  await cbox.fill(`/assign ${key}-2 @${bobur.email}`);
  await cbox.press("Enter");
  await expect(a.getByText(`👤 ${key}-2 is assigned to Bobur`)).toBeVisible();
  await cbox.fill(`/status ${key}-2 in progress`);
  await cbox.press("Enter");
  await expect(a.getByText(new RegExp(`🔀 ${key}-2 is now In Progress`, "i"))).toBeVisible();
  await cbox.fill(`/status ${key}-2 Shipped`);
  await cbox.press("Enter");
  await expect(a.getByText(/No status "Shipped"/)).toBeVisible();
  const made = await ali.call("GET", `/api/v1/issues/${key}-2`);
  expect(made.status).toBe("in_progress");
  await b.goto(`/chat/${channel.id}`);
  await expect(b.getByRole("log", { name: "Messages" })).not.toContainText("Fix the flaky login test");

  // S73: Ali links the channel to the project; a new issue shows up as project news.
  await a.goto(`/chat/${channel.id}`);
  await a.getByRole("button", { name: "⚙ Settings" }).click();
  await a.getByLabel("Project news", { exact: true }).selectOption(key);
  await expect(a.getByLabel("Project news", { exact: true })).toHaveValue(key);
  await ali.call("POST", `/api/v1/projects/${key}/issues`, { title: "Announce the release", type: "task" });
  const news = a.getByRole("article", { name: /Project news/ });
  await expect(news).toContainText(`created ${key}-3`);
  await expect(news.getByRole("link", { name: `${key}-3 Announce the release` })).toBeVisible();
  await a.getByRole("button", { name: "⚙ Settings" }).click(); // close

  // S74: a message becomes an issue; its key is replied in the thread and the issue links back.
  await cbox.fill("The export button times out on big boards\nSeen on KYB board with 2k cards");
  await cbox.press("Enter");
  const msg = a.getByRole("article").filter({ hasText: "The export button times out" }).first();
  await msg.getByLabel("Create issue from message").selectOption({ label: `Create in ${key}` });
  await expect(msg.getByRole("button", { name: "1 reply" })).toBeVisible();
  const fromChat = await ali.call("GET", `/api/v1/issues/${key}-4`);
  expect(fromChat.title).toBe("The export button times out on big boards");
  expect(fromChat.description).toContain(`/chat/${channel.id}`);

  // S69: the owner renames the channel, makes Bobur an admin; Bobur archives it and it turns read-only for both.
  await a.goto(`/chat/${channel.id}`);
  await a.getByRole("button", { name: "⚙ Settings" }).click();
  const settings = a.getByRole("region", { name: "Channel settings" });
  await settings.getByLabel("Name").fill(`renamed-${id}`);
  await settings.getByRole("button", { name: "Save" }).click();
  await expect(a.getByRole("heading", { name: `# renamed-${id}` })).toBeVisible();
  await settings.getByRole("listitem").filter({ hasText: "Bobur" }).getByRole("button", { name: "Make admin" }).click();
  await expect(settings.getByRole("listitem").filter({ hasText: "Bobur" })).toContainText("admin");
  await b.goto(`/chat/${channel.id}`);
  await expect(b.getByRole("heading", { name: `# renamed-${id}` })).toBeVisible();
  await b.getByRole("button", { name: "⚙ Settings" }).click();
  await b.getByRole("button", { name: "Archive channel" }).click();
  await expect(b.getByText("This channel is archived")).toBeVisible();
  await expect(a.getByText("This channel is archived")).toBeVisible(); // live
  await expect(a.getByRole("textbox", { name: /^Message/ })).toHaveCount(0);
});
