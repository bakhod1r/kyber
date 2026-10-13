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
- **S66 Status and Do Not Disturb** — status emoji + text with optional expiry; DND until a time; while a
  Pomodoro is running I am in DND automatically. Others see status next to my name. AC: chat notifications are
  held during DND (still listed in Activity); status expires by itself.
- **S67 Scheduled messages and reminders** — post a message at a later time (edit/cancel before it is sent);
  `/remind me <when> <what>`. AC: sent by the background worker once, at most a minute late; only the author sees
  it before it is sent.
- **S68 Typing and read receipts** — "X is typing…" (typing API exists since S51) in the UI; in DMs "Seen" under
  my last message. AC: receipts only in DMs; never for channels.

## 2. Channels

- **S69 Channel management** — rename, archive/unarchive (read-only when archived), owner and admins of a channel
  (owner = creator). AC: only owner/admins rename, archive, remove members.
- **S70 User groups** — `@backend` style groups per workspace; mentioning a group notifies its members. AC: a
  group mention only notifies members who may read the conversation.
- **S71 Sidebar sections and notification levels** — my own sections (e.g. "Project", "Team") with drag to
  reorder; per conversation: all / mentions only / muted. AC: per user; muted conversations never notify.

## 3. Jira integration

- **S72 Slash commands** — `/issue <project> <title>`, `/assign KEY @user`, `/status KEY <status>`; the answer is
  an ephemeral message only I see. AC: the same permissions as the issue API.
- **S73 Project channels** — link a channel to a project; issue created, status changed, sprint started/completed
  are posted there by the system (outbox handlers). AC: only people who may browse the project see the details.
- **S74 Issue from a message** — "Create issue" in a message's menu: the message becomes the description with a
  link back; the issue key is shown on the message. AC: needs create permission in the chosen project.

## 4. Huddles

- **S75 Huddle chat and reactions** — chat inside the huddle (stored as a thread of the huddle_started message),
  raise hand ✋ and emoji reactions shown over the tile. AC: reactions are ephemeral (not stored).
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
