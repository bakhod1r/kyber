# Sprint 12 — More from Slack

Source: product request (2026-10-13, "hammasi"). Builds on Sprint 11 (ADR-0009). Each story is planned
(scope + AC) before it starts, then built DDD + TDD with acceptance tests on memory and PostgreSQL, OpenAPI first.
Order: daily use first, then channel management, Jira integration, huddle extras, the rest.

## 1. Daily use

- ✅ **S64 Activity: unreads and mentions** — "🔔 Activity" in the chat sidebar: my chat mentions (unread first,
  from the Notify inbox) and conversations with unread messages. Opening a conversation reads my mentions in it
  (`POST /notifications/read-all?conversation_id=`).
- ✅ **S65 Saved for later** — save a message, optionally with a reminder time; `GET /chat/saved`; the reminder
  arrives as a notification at its time. AC: per user; a deleted message shows as deleted; unsave.
  **Done:** `PUT/DELETE /messages/{id}/save`, migration 0055 (RLS), `chat.reminder` outbox event → Notify kind
  `reminder`, background job (once across replicas); 🔖 menu on messages and "Saved for later" in Activity, API 0.46.0.
  Also fixed: chat notifications in the bell now open the conversation.
- ✅ **S66 Status and Do Not Disturb** — status emoji + text with optional expiry; DND until a time; while a
  Pomodoro is running I am in DND automatically. Others see status next to my name. AC: chat notifications are
  held during DND (still listed in Activity); status expires by itself.
  **Done:** `PUT /me/status`, `GET /chat/statuses`, migration 0056 (RLS); Notify's Quiet ACL (DND or a running
  Pomodoro) keeps chat mentions in-app only; status presets, "clear after", 🌙 next to names, API 0.47.0.
- ✅ **S67 Scheduled messages and reminders** — post a message at a later time (edit/cancel before it is sent);
  `/remind me <when> <what>`. AC: sent by the background worker once, at most a minute late; only the author sees
  it before it is sent.
  **Done:** `POST /conversations/{id}/scheduled`, `GET /chat/scheduled`, `PATCH/DELETE /scheduled/{id}`,
  `POST /conversations/{id}/reminders`; migration 0057 (RLS); the job posts each once (message `ref`), drops it
  if the author left; reminders reuse `chat.reminder`. UI: 🕒 "Send later", `/remind me in|tomorrow|at …`,
  "Scheduled" in Activity. API 0.48.0.
- ✅ **S68 Typing and read receipts** — "X is typing…" (typing API exists since S51) in the UI; in DMs "Seen" under
  my last message. AC: receipts only in DMs; never for channels.
  **Done:** `GET /conversations/{id}/receipts`, live `conversation.read` to the others in a DM; "✓ Seen" under my
  latest message. API 0.49.0.

## 2. Channels

- ✅ **S69 Channel management** — rename, archive/unarchive (read-only when archived), owner and admins of a channel
  (owner = creator). AC: only owner/admins rename, archive, remove members.
  **Done:** `PATCH /conversations/{id}`, `POST …/archive|unarchive`, `PUT/DELETE …/admins/{user}`,
  `DELETE …/members/{user}`, migration 0058; archived channels refuse every write (and scheduled messages to them
  are dropped); live `conversation.updated`; "⚙ Settings" panel. API 0.50.0.
- ✅ **S70 User groups** — `@backend` style groups per workspace; mentioning a group notifies its members. AC: a
  group mention only notifies members who may read the conversation.
  **Done:** `GET/POST /groups`, `PATCH/DELETE /groups/{id}`, migration 0059 (RLS); Notify resolves `@handle`
  through an ACL to Chat (once per person, never the author); "User groups" in Activity; @handles highlighted.
  API 0.51.0.
- ✅ **S71 Sidebar sections and notification levels** — my own sections (e.g. "Project", "Team") with drag to
  reorder; per conversation: all / mentions only / muted. AC: per user; muted conversations never notify.
  **Done:** `PUT /conversations/{id}/prefs`, `GET /chat/prefs`, migration 0060 (RLS, notification kind `message`);
  Notify skips muted, tells followers (level all) every message once; sidebar sections (sorted by name — drag to
  reorder left for later), ⋯ menu per conversation. API 0.52.0.

## 3. Jira integration

- ✅ **S72 Slash commands** — `/issue <project> <title>`, `/assign KEY @user`, `/status KEY <status>`; the answer is
  an ephemeral message only I see. AC: the same permissions as the issue API.
  **Done:** Chat parses the commands; the app shell runs them through the Issue/Project clients (the user's own
  permissions, no new endpoint) and the answer is a line only the author sees.
- ✅ **S73 Project channels** — link a channel to a project; issue created, status changed, sprint started/completed
  are posted there by the system (outbox handlers). AC: only people who may browse the project see the details.
  **Done:** `PUT /conversations/{id}/project` (owner/admins who may browse it), migration 0061, relay handlers post
  `project_event` system messages once per event and channel; the body names issues by key only, details come from
  the permission-filtered preview. "Project news" in channel settings. API 0.53.0.
- ✅ **S74 Issue from a message** — "Create issue" in a message's menu: the message becomes the description with a
  link back; the issue key is shown on the message. AC: needs create permission in the chosen project.
  **Done:** "📋 Issue…" on a message; the app shell creates it through the Issue API (the user's own permission),
  first line → title, message + chat link → description; the key is replied in the message's thread.

## 4. Huddles

- ✅ **S75 Huddle chat and reactions** — chat inside the huddle (stored as a thread of the huddle_started message),
  raise hand ✋ and emoji reactions shown over the tile. AC: reactions are ephemeral (not stored).
  **Done:** `PUT /calls/{id}/hand` (stored on the participant, migration 0062), `POST /calls/{id}/reactions` →
  live `call.reaction`; "💬 Huddle chat" is the thread of the call's "started a huddle" message, composed by the app
  shell. API 0.54.0.
- **S76 Join a meeting's huddle** — a meeting in the Time table can be linked to a channel; "Join" opens its
  huddle. AC: only for people invited to the meeting who may read the channel.
- **S77 Recording transcripts (optional)** — a transcript and summary for a recording when a speech-to-text
  service is configured (off by default; no data leaves the server without it).

## 5. Rest

- **S78 Clips** — record a short video or voice message (≤ 5 min) and post it.
- **S79 Docs in channels** — attach docs to a channel (like Slack canvases); shown in the channel header.
- **S80 Workflows** — simple automations: on member joined / on schedule → post a message.
- **S81 Incoming webhooks and bot tokens** — a channel webhook URL (secret, rotatable) that posts messages, e.g.
  from CI. AC: the secret is stored hashed and never shown again; rate limited.
