import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { createContext, type FormEvent, type KeyboardEvent, type ReactNode, useContext, useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { identityUseCases } from "../../identity/application";
import { projectUseCases } from "../../project/application";
import {
  applyEvent, canManage, type ChannelWorkflow, type ChatEvent, localTime, utcMinute, type ConversationPrefs, type Level, moveSection, sections, chatUseCases, type Conversation, clearAfter, conversationTitle, fileSize, MAX_FILE, COMMAND_HELP, clipName, type ClipKind, type ClipRecorder, MAX_CLIP_MS, type Message, parseCommand, startClip, parseRemind, type SlashCommand, remindPresets, type ScheduledMessage, type Receipt, seen, type SavedItem, splitMentions, STATUS_PRESETS, type UserGroup, type UserStatus, useChatStream,
} from "../application";
import { msg, useT } from "../../../shared/i18n/i18n";

const PAGE = 50;
const TYPING_MS = 4000;
const QUICK = ["👍", "🎉", "👀", "✅"];
const historyKey = (id: string) => ["chat", "history", id];
const threadKey = (id: string) => ["chat", "thread", id];
const time = (iso: string) => new Date(iso).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });

/** People the user works with (project members), by id → name. */
function usePeople() {
  const projects = useQuery({ queryKey: ["projects"], queryFn: projectUseCases.projects });
  const lists = useQueries({
    queries: (projects.data ?? []).map((p) => ({ queryKey: ["members", p.key], queryFn: () => projectUseCases.members(p.key) })),
  });
  const bots = useQuery({ queryKey: ["chat", "bots"], queryFn: chatUseCases.bots });
  const names = new Map<string, string>();
  for (const q of lists) for (const m of q.data ?? []) names.set(m.user_id, m.name);
  for (const b of bots.data ?? []) names.set(b.user_id, b.name); // S84
  return names;
}

/** People's statuses by id (S66), for the names shown in chat. */
const Statuses = createContext<Map<string, UserStatus>>(new Map());

/** A name with its status emoji (and 🌙 in Do Not Disturb). */
function Who({ id, names }: { id: string; names: Map<string, string> }) {
  const t = useT();
  const s = useContext(Statuses).get(id);
  return (
    <>
      <b>{names.get(id) ?? t("Someone")}</b>
      {s?.emoji && <span className="status-emoji" title={s.text}> {s.emoji}</span>}
      {s?.dnd && <span title={t("Do not disturb")} aria-label={t("Do not disturb")}> 🌙</span>}
    </>
  );
}

/** A slot the app shell fills with other contexts' UIs for the open conversation (huddle, whiteboards). */
export type ConversationSlot = (conversationId: string, me: string, names: Map<string, string>) => ReactNode;

/** Team chat (S53, ADR-0009): channels and DMs on the left, the open conversation and its threads on the right. */
/** Runs /issue, /assign and /status (S72) and answers with a line only the user sees. */
export type CommandRunner = (cmd: SlashCommand) => Promise<string>;
const Commands = createContext<CommandRunner | undefined>(undefined);
/** Makes an issue from a message (S74): returns its key. Provided by the app shell. */
export type IssueMaker = (body: string, project: string, link: string) => Promise<string>;
const IssueMakerContext = createContext<IssueMaker | undefined>(undefined);

export function ChatPage({ extras, activity, commands, issueMaker }: {
  extras?: ConversationSlot; /** Other contexts' activity (mentions), shown on the Activity view. */ activity?: ReactNode;
  commands?: CommandRunner; issueMaker?: IssueMaker;
}) {
  const t = useT();
  const { id } = useParams();
  const qc = useQueryClient();
  const me = useQuery({ queryKey: ["me"], queryFn: identityUseCases.me });
  const names = usePeople();
  const conversations = useQuery({ queryKey: ["chat", "conversations"], queryFn: chatUseCases.conversations });
  const [typing, setTyping] = useState<Record<string, { user: string; until: number }[]>>({});
  const [thread, setThread] = useState<string | null>(null);

  useChatStream((e: ChatEvent) => {
    if (e.type === "conversation.updated") {
      void qc.invalidateQueries({ queryKey: ["chat", "conversations"] });
      void qc.invalidateQueries({ queryKey: ["chat", "bookmarks", e.conversation_id] });
      return;
    }
    if (e.type === "conversation.read") {
      qc.setQueryData<Receipt[]>(["chat", "receipts", e.conversation_id], (list) =>
        list ? [...list.filter((r) => r.user_id !== e.user_id), { user_id: e.user_id, read_at: e.at }] : list);
      return;
    }
    if (e.type === "typing") {
      setTyping((t) => ({ ...t, [e.conversation_id]: [...(t[e.conversation_id] ?? []).filter((x) => x.user !== e.user_id), { user: e.user_id, until: Date.now() + TYPING_MS }] }));
      return;
    }
    const conv = e.message.conversation_id;
    qc.setQueryData<Message[]>(historyKey(conv), (list) => (list ? applyEvent(list, conv, e) : list));
    // A history fetch still in flight would answer without this message and overwrite it: fetch again (the
    // server stored the message before announcing it).
    if (qc.isFetching({ queryKey: historyKey(conv) })) void qc.invalidateQueries({ queryKey: historyKey(conv) });
    if (e.message.thread_id) void qc.invalidateQueries({ queryKey: threadKey(e.message.thread_id) });
    if (e.type === "message.pinned") void qc.invalidateQueries({ queryKey: ["chat", "pins", conv] });
    if (e.type === "message.posted") {
      setTyping((t) => ({ ...t, [conv]: (t[conv] ?? []).filter((x) => x.user !== e.message.author_id) }));
      if (conv === id && !e.message.thread_id) void chatUseCases.markRead(conv);
      else void qc.invalidateQueries({ queryKey: ["chat", "conversations"] });
    }
  }, () => void qc.invalidateQueries({ queryKey: ["chat"] }));

  const current = conversations.data?.find((c) => c.id === id);
  const myId = me.data?.id ?? "";
  const title = (c: Conversation) => conversationTitle(c, names, myId);
  const people = [...new Set([...names.keys(), myId].filter(Boolean))].sort();
  const statuses = useQuery({ queryKey: ["chat", "statuses", people], queryFn: () => chatUseCases.statuses(people), refetchInterval: 60_000 });
  const statusMap = new Map((statuses.data ?? []).map((s) => [s.user_id, s]));
  const typers = (typing[id ?? ""] ?? []).filter((x) => x.until > Date.now() && x.user !== myId).map((x) => names.get(x.user) ?? t("Someone"));

  return (
    <Commands.Provider value={commands}>
    <IssueMakerContext.Provider value={issueMaker}>
    <Statuses.Provider value={statusMap}>
    <div className="chat">
      <Sidebar conversations={conversations.data ?? []} current={id} title={title} names={names} me={myId} />
      {id ? (
        <ConversationView key={id} id={id} conversation={current} title={current ? title(current) : ""} names={names} me={myId}
          typers={typers} onThread={setThread} extras={current?.member && myId ? extras?.(id, myId, names) : undefined} />
      ) : (
        <Activity conversations={conversations.data ?? []} title={title} activity={activity} names={names} me={myId} />
      )}
      {thread && <ThreadPanel parent={thread} names={names} me={myId} onClose={() => setThread(null)} />}
    </div>
    </Statuses.Provider>
    </IssueMakerContext.Provider>
    </Commands.Provider>
  );
}

/** Set my status and Do Not Disturb (S66). */
function MyStatus({ me }: { me: string }) {
  const t = useT();
  const qc = useQueryClient();
  const mine = useContext(Statuses).get(me);
  const [open, setOpen] = useState(false);
  const [emoji, setEmoji] = useState("");
  const [text, setText] = useState("");
  const [clear, setClear] = useState(1);
  const [dnd, setDnd] = useState("");
  const set = useMutation({
    mutationFn: (s: { emoji: string; text: string; expires_at: string | null; dnd_until: string | null }) => chatUseCases.setStatus(s),
    onSuccess: () => {
      setOpen(false);
      void qc.invalidateQueries({ queryKey: ["chat", "statuses"] });
    },
  });
  const save = (e: FormEvent) => {
    e.preventDefault();
    const now = new Date();
    const until = dnd ? new Date(now.getTime() + Number(dnd) * 60_000).toISOString() : null;
    set.mutate({ emoji, text, expires_at: clearAfter(now)[clear]!.at?.toISOString() ?? null, dnd_until: until });
  };
  return (
    <div className="chat-status">
      <button className="ghost" onClick={() => { setOpen(!open); setEmoji(mine?.emoji ?? ""); setText(mine?.text ?? ""); }}>
        {mine?.emoji || mine?.text ? `${mine.emoji} ${mine.text}` : t("😀 Set a status")}{mine?.dnd ? t(" · 🌙 Do not disturb") : ""}
      </button>
      {open && (
        <form onSubmit={save} aria-label={t("Status")} className="chat-new">
          <div className="row">
            {STATUS_PRESETS.map((p) => (
              <button type="button" key={p.text} className="ghost" title={p.text} onClick={() => { setEmoji(p.emoji); setText(p.text); }}>{p.emoji}</button>
            ))}
          </div>
          <label>{t("Emoji")} <input value={emoji} onChange={(e) => setEmoji(e.target.value)} maxLength={8} style={{ width: "4em" }} /></label>
          <label>{t("Status")} <input value={text} onChange={(e) => setText(e.target.value)} maxLength={100} placeholder={t("What's your status?")} /></label>
          <label>{t("Clear after")}{" "}
            <select value={clear} onChange={(e) => setClear(Number(e.target.value))}>
              {clearAfter(new Date()).map((c, i) => <option key={c.label} value={i}>{c.label}</option>)}
            </select>
          </label>
          <label>{t("Do not disturb")}{" "}
            <select value={dnd} onChange={(e) => setDnd(e.target.value)}>
              <option value="">{t("Off")}</option>
              <option value="30">{t("30 minutes")}</option>
              <option value="60">{t("1 hour")}</option>
              <option value="240">{t("4 hours")}</option>
              <option value="720">{t("12 hours")}</option>
            </select>
          </label>
          <div className="row">
            <button>{t("Save")}</button>
            <button type="button" className="ghost" onClick={() => set.mutate({ emoji: "", text: "", expires_at: null, dnd_until: null })}>{t("Clear")}</button>
          </div>
          {set.error && <p role="alert" className="error">{set.error.message}</p>}
        </form>
      )}
    </div>
  );
}

/** S78: record a short video or voice message (at most five minutes) and post it. */
function ClipButton({ onClip }: { onClip: (f: File) => void }) {
  const t = useT();
  const [rec, setRec] = useState<{ kind: ClipKind; r: ClipRecorder; started: number } | null>(null);
  const [now, setNow] = useState(0);
  const [error, setError] = useState("");
  const preview = useRef<HTMLVideoElement>(null);
  const stop = async () => {
    if (!rec) return;
    const blob = await rec.r.stop();
    setRec(null);
    if (blob.size > 0) onClip(new File([blob], clipName(rec.kind, new Date()), { type: blob.type }));
  };
  useEffect(() => {
    if (!rec) return;
    if (preview.current) preview.current.srcObject = rec.kind === "video" ? rec.r.stream : null;
    const t = setInterval(() => setNow(Date.now()), 250);
    return () => clearInterval(t);
  }, [rec]);
  useEffect(() => {
    if (rec && now - rec.started >= MAX_CLIP_MS) void stop();
  });
  const start = async (kind: ClipKind) => {
    setError("");
    try {
      setRec({ kind, r: await startClip(kind), started: Date.now() });
      setNow(Date.now());
    } catch (e) {
      setError(e instanceof Error ? e.message : t("No camera or microphone"));
    }
  };
  if (rec) {
    const secs = Math.max(0, Math.floor((now - rec.started) / 1000));
    return (
      <div className="chat-clip" role="group" aria-label={t("Recording a clip")}>
        {rec.kind === "video" && <video ref={preview} autoPlay muted playsInline className="chat-clip-preview" />}
        <span className="huddle-recording">● {Math.floor(secs / 60)}:{String(secs % 60).padStart(2, "0")} / 5:00</span>
        <button onClick={() => void stop()}>{t("Stop and send")}</button>
      </div>
    );
  }
  return (
    <>
      <select className="ghost" aria-label={t("Record a clip")} value="" onChange={(e) => e.target.value && void start(e.target.value as ClipKind)}>
        <option value="">🎬</option>
        <option value="video">{t("Video clip")}</option>
        <option value="voice">{t("Voice clip")}</option>
      </select>
      {error && <span role="alert" className="error"> {error}</span>}
    </>
  );
}

/** S74: "Create issue" from a message; the new key is replied in its thread. */
function IssueFromMessage({ m, onDone }: { m: Message; onDone: (m: Message) => void }) {
  const t = useT();
  const maker = useContext(IssueMakerContext);
  const qc = useQueryClient();
  const projects = useQuery({ queryKey: ["projects"], queryFn: projectUseCases.projects, enabled: !!maker });
  const make = useMutation({
    mutationFn: async (project: string) => {
      const key = await maker!(m.body, project, `${window.location.origin}/chat/${m.conversation_id}`);
      await chatUseCases.reply(m.id, `📋 Created ${key} from this message`);
      return key;
    },
    onSuccess: () => {
      onDone({ ...m, reply_count: m.reply_count + 1 });
      void qc.invalidateQueries({ queryKey: ["chat", "thread", m.id] });
    },
  });
  if (!maker || (projects.data?.length ?? 0) === 0) return null;
  return (
    <>
      <select className="ghost" aria-label={t("Create issue from message")} value="" onChange={(e) => e.target.value && make.mutate(e.target.value)}>
        <option value="">{t("📋 Issue…")}</option>
        {projects.data!.map((p) => <option key={p.key} value={p.key}>{t("Create in {key}", { key: p.key })}</option>)}
      </select>
      {make.error && <span role="alert" className="error"> {make.error.message}</span>}
    </>
  );
}

/** Invite someone (or a bot) into the channel. */
function InviteMember({ c, names, onDone }: { c: Conversation; names: Map<string, string>; onDone: () => void }) {
  const t = useT();
  const invite = useMutation({ mutationFn: (u: string) => chatUseCases.invite(c.id, u), onSuccess: onDone });
  const others = [...names].filter(([u]) => !c.member_ids.includes(u));
  if (others.length === 0) return null;
  return (
    <label>
      {t("Invite")}{" "}
      <select aria-label={t("Invite to channel")} value="" onChange={(e) => e.target.value && invite.mutate(e.target.value)}>
        <option value="">{t("Someone…")}</option>
        {others.map(([u, n]) => <option key={u} value={u}>{n}</option>)}
      </select>
      {invite.error && <span role="alert" className="error"> {invite.error.message}</span>}
    </label>
  );
}

/** S84: bots — another system's user that posts through the API with its own token (shown once). */
function Bots() {
  const t = useT();
  const qc = useQueryClient();
  const list = useQuery({ queryKey: ["chat", "bots"], queryFn: chatUseCases.bots });
  const [name, setName] = useState("");
  const [shown, setShown] = useState<{ name: string; token: string } | null>(null);
  const create = useMutation({
    mutationFn: () => chatUseCases.createBot(name),
    onSuccess: (b) => { setShown({ name: b.name, token: b.token ?? "" }); setName(""); void qc.invalidateQueries({ queryKey: ["chat", "bots"] }); },
  });
  return (
    <>
      <h2>{t("🤖 Bots")}</h2>
      <ul aria-label={t("Bots")}>{(list.data ?? []).map((b) => <li key={b.user_id}>{b.name}</li>)}</ul>
      {shown && (
        <div className="panel" role="status" aria-label={t("Bot token")}>
          <p>{t("Token for {name} — copy it now, it is not shown again. Invite the bot to channels, then:", { name: shown.name })}</p>
          <input readOnly value={shown.token} aria-label={t("Bot token value")} style={{ width: "100%" }} onFocus={(e) => e.currentTarget.select()} />
          <pre className="muted">{`curl -X POST -H 'Authorization: Bearer ${shown.token}' -H 'Content-Type: application/json' \\
  -d '{"body":"Build passed"}' ${window.location.origin}/api/v1/conversations/<channel id>/messages`}</pre>
          <button className="ghost" onClick={() => setShown(null)}>{t("Done")}</button>
        </div>
      )}
      <form className="row" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <input aria-label={t("Bot name")} placeholder="CI" value={name} onChange={(e) => setName(e.target.value)} maxLength={100} />
        <button disabled={!name.trim()}>{t("Add bot")}</button>
      </form>
      {create.error && <p role="alert" className="error">{create.error.message}</p>}
    </>
  );
}

/** S81: incoming webhooks — other systems (CI, monitoring) post here through a secret URL, shown once. */
function Webhooks({ c }: { c: Conversation }) {
  const t = useT();
  const qc = useQueryClient();
  const key = ["chat", "webhooks", c.id];
  const list = useQuery({ queryKey: key, queryFn: () => chatUseCases.webhooks(c.id) });
  const done = () => qc.invalidateQueries({ queryKey: key });
  const [name, setName] = useState("");
  const [shown, setShown] = useState<{ name: string; url: string } | null>(null);
  const abs = (url: string) => `${window.location.origin}${url}`;
  const create = useMutation({
    mutationFn: () => chatUseCases.createWebhook(c.id, name),
    onSuccess: (h) => { setShown({ name: h.name, url: abs(h.url ?? "") }); setName(""); void done(); },
  });
  const rotate = useMutation({
    mutationFn: (h: { id: string; name: string }) => chatUseCases.rotateWebhook(h.id).then((r) => ({ name: h.name, url: abs(r.url) })),
    onSuccess: setShown,
  });
  const remove = useMutation({ mutationFn: (id: string) => chatUseCases.removeWebhook(id), onSuccess: done });
  return (
    <div role="group" aria-label={t("Webhooks")}>
      <h3>{t("🔌 Incoming webhooks")}</h3>
      <ul>
        {(list.data ?? []).map((h) => (
          <li key={h.id}>
            {h.name}{" "}
            <button className="ghost" onClick={() => rotate.mutate(h)}>{t("New URL")}</button>
            <button className="ghost" onClick={() => remove.mutate(h.id)}>{t("Remove")}</button>
          </li>
        ))}
      </ul>
      {shown && (
        <div className="panel" role="status" aria-label={t("Webhook URL")}>
          <p>{t("Copy the URL for {name} now — it is not shown again:", { name: shown.name })}</p>
          <input readOnly value={shown.url} aria-label={t("Webhook URL value")} onFocus={(e) => e.currentTarget.select()} style={{ width: "100%" }} />
          <pre className="muted">{`curl -X POST -H 'Content-Type: application/json' -d '{"text":"Build passed"}' ${shown.url}`}</pre>
          <button className="ghost" onClick={() => setShown(null)}>{t("Done")}</button>
        </div>
      )}
      <form className="row" onSubmit={(e) => { e.preventDefault(); create.mutate(); }}>
        <input aria-label={t("Webhook name")} placeholder="CI" value={name} onChange={(e) => setName(e.target.value)} maxLength={40} />
        <button disabled={!name.trim()}>{t("Add webhook")}</button>
      </form>
      {(create.error ?? rotate.error) && <p role="alert" className="error">{(create.error ?? rotate.error)!.message}</p>}
    </div>
  );
}

const DAY_NAMES = [msg("Sun"), msg("Mon"), msg("Tue"), msg("Wed"), msg("Thu"), msg("Fri"), msg("Sat")];

/** S80: a welcome when someone joins, or a message on chosen days at a time. */
function Workflows({ c }: { c: Conversation }) {
  const t = useT();
  const qc = useQueryClient();
  const key = ["chat", "workflows", c.id];
  const list = useQuery({ queryKey: key, queryFn: () => chatUseCases.workflows(c.id) });
  const done = () => qc.invalidateQueries({ queryKey: key });
  const [trigger, setTrigger] = useState<ChannelWorkflow["trigger"]>("member_joined");
  const [at, setAt] = useState("09:30");
  const [days, setDays] = useState([1, 2, 3, 4, 5]);
  const [text, setText] = useState("");
  const add = useMutation({
    mutationFn: () => chatUseCases.addWorkflow(c.id, trigger === "schedule" ? { trigger, minute: utcMinute(at), days, text } : { trigger, text }),
    onSuccess: () => { setText(""); void done(); },
  });
  const remove = useMutation({ mutationFn: (id: string) => chatUseCases.removeWorkflow(id), onSuccess: done });
  return (
    <div aria-label={t("Workflows")} role="group">
      <h3>{t("🤖 Workflows")}</h3>
      <ul>
        {(list.data ?? []).map((w) => (
          <li key={w.id}>
            {w.trigger === "member_joined" ? t("When someone joins") : t("{days} at {time}", { days: w.days.map((d) => t(DAY_NAMES[d]!)).join(", "), time: localTime(w.minute) })}: <i>{w.text}</i>{" "}
            <button className="ghost" onClick={() => remove.mutate(w.id)}>{t("Remove")}</button>
          </li>
        ))}
      </ul>
      <form className="row" onSubmit={(e) => { e.preventDefault(); add.mutate(); }}>
        <select aria-label={t("Workflow trigger")} value={trigger} onChange={(e) => setTrigger(e.target.value as ChannelWorkflow["trigger"])}>
          <option value="member_joined">{t("When someone joins")}</option>
          <option value="schedule">{t("On a schedule")}</option>
        </select>
        {trigger === "schedule" && (
          <>
            <input type="time" aria-label={t("At")} value={at} onChange={(e) => setAt(e.target.value)} />
            {DAY_NAMES.map((d, i) => (
              <label key={d}><input type="checkbox" checked={days.includes(i)} onChange={(e) => setDays((x) => (e.target.checked ? [...x, i] : x.filter((y) => y !== i)))} />{t(d)}</label>
            ))}
          </>
        )}
        <input aria-label={t("Workflow message")} value={text} onChange={(e) => setText(e.target.value)} placeholder={trigger === "member_joined" ? t("Welcome {name}!") : t("Standup in 5 minutes")} />
        <button disabled={!text.trim()}>{t("Add workflow")}</button>
      </form>
      {add.error && <p role="alert" className="error">{add.error.message}</p>}
    </div>
  );
}

/** S73: post a project's news (issues created and moved, sprints) into the channel. */
function ProjectLink({ c, onChange }: { c: Conversation; onChange: () => void }) {
  const t = useT();
  const projects = useQuery({ queryKey: ["projects"], queryFn: projectUseCases.projects });
  const link = useMutation({ mutationFn: (key: string) => chatUseCases.linkProject(c.id, key), onSuccess: onChange });
  return (
    <label>
      {t("📋 Project news")}{" "}
      <select aria-label={t("Project news")} value={c.project_key ?? ""} disabled={c.archived} onChange={(e) => link.mutate(e.target.value)}>
        <option value="">{t("None")}</option>
        {(projects.data ?? []).map((p) => <option key={p.key} value={p.key}>{p.key} — {p.name}</option>)}
      </select>
      {link.error && <span role="alert" className="error"> {link.error.message}</span>}
    </label>
  );
}

/** Issues named in a message, for readers who may browse them. */
function IssuePreviews({ m }: { m: Message }) {
  if (m.issues.length === 0) return null;
  return (
    <ul className="chat-issues">
      {m.issues.map((p) => (
        <li key={p.key}>
          <Link to={`/projects/${p.key.split("-")[0]}?issue=${p.key}`} aria-label={`${p.key} ${p.title}`}>
            <b>{p.key}</b> {p.title} <span className="muted">{p.status}</span>
          </Link>
        </li>
      ))}
    </ul>
  );
}

/** Owner and admins: rename, archive, admins and members (S69). Everyone else sees who runs the channel. */
function ChannelSettings({ c, me, names }: { c: Conversation; me: string; names: Map<string, string> }) {
  const t = useT();
  const qc = useQueryClient();
  const [name, setName] = useState(c.name);
  const [topic, setTopic] = useState(c.topic);
  const done = () => qc.invalidateQueries({ queryKey: ["chat", "conversations"] });
  const act = useMutation({ mutationFn: (f: () => Promise<Conversation>) => f(), onSuccess: done });
  const manager = canManage(c, me);
  const owner = c.owner_id === me;
  const who = (u: string) => (u === me ? t("You") : (names.get(u) ?? t("Someone")));
  return (
    <section className="chat-settings panel" aria-label={t("Channel settings")}>
      {manager && (
        <form className="row" onSubmit={(e) => { e.preventDefault(); act.mutate(() => chatUseCases.rename(c.id, name, topic)); }}>
          <label>{t("Name")} <input value={name} onChange={(e) => setName(e.target.value)} disabled={c.archived} /></label>
          <label>{t("Topic")} <input value={topic} onChange={(e) => setTopic(e.target.value)} disabled={c.archived} /></label>
          <button disabled={c.archived}>{t("Save")}</button>
          {c.archived
            ? <button type="button" className="ghost" onClick={() => act.mutate(() => chatUseCases.unarchive(c.id))}>{t("Unarchive")}</button>
            : <button type="button" className="danger" onClick={() => act.mutate(() => chatUseCases.archive(c.id))}>{t("Archive channel")}</button>}
        </form>
      )}
      {manager && <ProjectLink c={c} onChange={() => void done()} />}
      {manager && <Workflows c={c} />}
      {manager && <Webhooks c={c} />}
      {!manager && c.project_key && <p className="muted">{t("📋 Posts news of {key}", { key: c.project_key })}</p>}
      {!c.archived && c.member && <InviteMember c={c} names={names} onDone={() => void done()} />}
      <ul aria-label={t("Channel members")}>
        {c.member_ids.map((u) => (
          <li key={u}>
            {who(u)} {u === c.owner_id ? <span className="muted">{t("· owner")}</span> : (c.admin_ids ?? []).includes(u) ? <span className="muted">{t("· admin")}</span> : null}
            {owner && u !== c.owner_id && ((c.admin_ids ?? []).includes(u)
              ? <button className="ghost" onClick={() => act.mutate(() => chatUseCases.removeAdmin(c.id, u))}>{t("Remove admin")}</button>
              : <button className="ghost" onClick={() => act.mutate(() => chatUseCases.makeAdmin(c.id, u))}>{t("Make admin")}</button>)}
            {manager && !c.archived && u !== c.owner_id && u !== me && (
              <button className="ghost" onClick={() => act.mutate(() => chatUseCases.removeMember(c.id, u))}>{t("Remove")}</button>
            )}
          </li>
        ))}
      </ul>
      {act.error && <p role="alert" className="error">{act.error.message}</p>}
    </section>
  );
}

/** User groups of the workspace (S70): make one, change its people, delete your own. */
function Groups({ names, me }: { names: Map<string, string>; me: string }) {
  const t = useT();
  const qc = useQueryClient();
  const groups = useQuery({ queryKey: ["chat", "groups"], queryFn: chatUseCases.groups });
  const [editing, setEditing] = useState<UserGroup | "new" | null>(null);
  const [handle, setHandle] = useState("");
  const [name, setName] = useState("");
  const [people, setPeople] = useState<string[]>([]);
  const done = () => {
    setEditing(null);
    void qc.invalidateQueries({ queryKey: ["chat", "groups"] });
  };
  const save = useMutation({
    mutationFn: () => {
      const g = { handle, name, member_ids: people };
      return editing === "new" || !editing ? chatUseCases.createGroup(g) : chatUseCases.updateGroup(editing.id, g);
    },
    onSuccess: done,
  });
  const remove = useMutation({ mutationFn: (id: string) => chatUseCases.deleteGroup(id), onSuccess: done });
  const open = (g: UserGroup | "new") => {
    setEditing(g);
    setHandle(g === "new" ? "" : g.handle);
    setName(g === "new" ? "" : g.name);
    setPeople(g === "new" ? [] : g.member_ids);
  };
  const who = (u: string) => (u === me ? t("You") : (names.get(u) ?? t("Someone")));
  return (
    <>
      <h2>{t("User groups")}</h2>
      <ul aria-label={t("User groups")}>
        {(groups.data ?? []).map((g) => (
          <li key={g.id}>
            <b>@{g.handle}</b> {g.name && <span className="muted">{g.name}</span>} · {g.member_ids.map(who).join(", ") || t("nobody yet")}{" "}
            <button className="ghost" onClick={() => open(g)}>{t("Edit")}</button>
            {g.created_by === me && <button className="ghost" onClick={() => remove.mutate(g.id)}>{t("Delete")}</button>}
          </li>
        ))}
      </ul>
      <button className="ghost" onClick={() => open("new")}>{t("New group")}</button>
      {editing && (
        <form className="chat-new" aria-label={t("Group")} onSubmit={(e) => { e.preventDefault(); save.mutate(); }}>
          <label>{t("Handle")} @<input value={handle} onChange={(e) => setHandle(e.target.value)} placeholder="backend" required /></label>
          <label>{t("Name")} <input value={name} onChange={(e) => setName(e.target.value)} placeholder={t("Backend team")} /></label>
          <fieldset>
            <legend>{t("People")}</legend>
            {[...new Set([me, ...names.keys()])].filter(Boolean).map((u) => (
              <label key={u}>
                <input type="checkbox" checked={people.includes(u)}
                  onChange={(e) => setPeople((p) => (e.target.checked ? [...p, u] : p.filter((x) => x !== u)))} /> {who(u)}
              </label>
            ))}
          </fieldset>
          <button>{t("Save group")}</button> <button type="button" className="ghost" onClick={() => setEditing(null)}>{t("Cancel")}</button>
          {save.error && <p role="alert" className="error">{save.error.message}</p>}
        </form>
      )}
    </>
  );
}

const savedKey = ["chat", "saved"];
const scheduledKey = ["chat", "scheduled"];

/** My scheduled messages, soonest first, with Cancel (S67). */
function ScheduledList({ title }: { title: (conversationId: string) => string }) {
  const t = useT();
  const qc = useQueryClient();
  const list = useQuery({ queryKey: scheduledKey, queryFn: chatUseCases.scheduled, refetchInterval: 30_000 });
  const cancel = useMutation({ mutationFn: (id: string) => chatUseCases.cancelScheduled(id), onSuccess: () => qc.invalidateQueries({ queryKey: scheduledKey }) });
  const items: ScheduledMessage[] = list.data ?? [];
  if (items.length === 0) return null;
  return (
    <>
      <h2>{t("Scheduled")}</h2>
      <ul aria-label={t("Scheduled messages")}>
        {items.map((x) => (
          <li key={x.id}>
            <Link to={`/chat/${x.conversation_id}`}>{title(x.conversation_id) || t("Conversation")}</Link>{" "}
            <span className="muted">🕒 {new Date(x.send_at).toLocaleString()}</span>
            <p>{x.body}</p>
            <button className="ghost" onClick={() => cancel.mutate(x.id)}>{t("Cancel")}</button>
          </li>
        ))}
      </ul>
    </>
  );
}

/** S65: save a message for later, optionally with a reminder; saved ones can be forgotten again. */
function SaveMenu({ m }: { m: Message }) {
  const t = useT();
  const qc = useQueryClient();
  const saved = useQuery({ queryKey: savedKey, queryFn: chatUseCases.saved });
  const mine = saved.data?.find((x) => x.message.id === m.id);
  const done = () => qc.invalidateQueries({ queryKey: savedKey });
  const save = useMutation({ mutationFn: (at: Date | null) => chatUseCases.save(m.id, at), onSuccess: done });
  const unsave = useMutation({ mutationFn: () => chatUseCases.unsave(m.id), onSuccess: done });
  if (mine) return <button className="ghost" aria-pressed onClick={() => unsave.mutate()} title={t("Saved — click to remove")}>{t("🔖 Saved")}</button>;
  const presets = remindPresets(new Date());
  return (
    <select className="ghost chat-save" aria-label={t("Save for later")} value="" onChange={(e) => {
      const v = e.target.value;
      if (v) save.mutate(v === "none" ? null : presets[Number(v)]!.at);
    }}>
      <option value="">{t("🔖 Save")}</option>
      <option value="none">{t("Save for later")}</option>
      {presets.map((p, i) => <option key={p.label} value={i}>{t("Remind me: {when}", { when: p.label.toLowerCase() })}</option>)}
    </select>
  );
}

/** My saved messages, most recently saved first (S65). */
function SavedList({ title }: { title: (conversationId: string) => string }) {
  const t = useT();
  const qc = useQueryClient();
  const saved = useQuery({ queryKey: savedKey, queryFn: chatUseCases.saved });
  const unsave = useMutation({ mutationFn: (id: string) => chatUseCases.unsave(id), onSuccess: () => qc.invalidateQueries({ queryKey: savedKey }) });
  const items: SavedItem[] = saved.data ?? [];
  return (
    <>
      <h2>{t("Saved for later")}</h2>
      {items.length === 0 ? <p className="muted">{t("Nothing saved. Use 🔖 on a message.")}</p> : (
        <ul aria-label={t("Saved messages")}>
          {items.map((x) => (
            <li key={x.message.id}>
              <Link to={`/chat/${x.message.conversation_id}`}>{title(x.message.conversation_id) || t("Conversation")}</Link>{" "}
              {x.remind_at && <span className="muted">{x.reminded ? t("⏰ reminded") : `⏰ ${new Date(x.remind_at).toLocaleString()}`}</span>}
              <p>{x.message.deleted ? <i className="muted">{t("deleted")}</i> : x.message.body}</p>
              <button className="ghost" onClick={() => unsave.mutate(x.message.id)}>{t("Remove")}</button>
            </li>
          ))}
        </ul>
      )}
    </>
  );
}

/** S64: where something waits for me — conversations with unread messages, and (from the shell) my mentions. */
function Activity({ conversations, title, activity, names, me }: {
  conversations: Conversation[]; title: (c: Conversation) => string; activity?: ReactNode; names: Map<string, string>; me: string;
}) {
  const t = useT();
  const unread = conversations.filter((c) => c.member && c.unread > 0);
  const titleOf = (id: string) => {
    const c = conversations.find((x) => x.id === id);
    return c ? title(c) : "";
  };
  return (
    <section className="chat-activity" aria-label={t("Activity")}>
      <h1>{t("Activity")}</h1>
      {activity}
      <ScheduledList title={titleOf} />
      <SavedList title={titleOf} />
      <h2>{t("Unread")}</h2>
      {unread.length === 0 ? <p className="muted">{t("You are all caught up.")}</p> : (
        <ul aria-label={t("Unread conversations")}>
          {unread.map((c) => (
            <li key={c.id}><Link to={`/chat/${c.id}`}>{title(c)}</Link> <span className="badge" aria-label={t("{n} unread", { n: c.unread })}>{c.unread}</span></li>
          ))}
        </ul>
      )}
      <Groups names={names} me={me} />
      <Bots />
    </section>
  );
}

function Sidebar({ conversations, current, title, names, me }: {
  conversations: Conversation[]; current?: string; title: (c: Conversation) => string; names: Map<string, string>; me: string;
}) {
  const t = useT();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [form, setForm] = useState<"channel" | "dm" | null>(null);
  const [name, setName] = useState("");
  const [priv, setPriv] = useState(false);
  const [picked, setPicked] = useState<string[]>([]);
  const opened = (c: Conversation) => {
    setForm(null);
    void qc.invalidateQueries({ queryKey: ["chat", "conversations"] });
    navigate(`/chat/${c.id}`);
  };
  const create = useMutation({ mutationFn: () => chatUseCases.createChannel({ name, topic: "", private: priv }), onSuccess: opened });
  const openDM = useMutation({ mutationFn: () => chatUseCases.openDM(picked), onSuccess: opened });
  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (form === "channel") create.mutate();
    else openDM.mutate();
  };
  const statuses = useContext(Statuses);
  const peer = (c: Conversation) => (c.kind === "dm" && c.member_ids.length === 2 ? statuses.get(c.member_ids.find((u) => u !== me) ?? "") : undefined);
  const prefs = useQuery({ queryKey: ["chat", "prefs"], queryFn: chatUseCases.prefs });
  const prefOf = (id: string): ConversationPrefs => prefs.data?.find((p) => p.conversation_id === id) ?? { conversation_id: id, level: "mentions", section: "" };
  const setPref = useMutation({
    mutationFn: ({ id, level, section }: { id: string; level: Level; section: string }) => chatUseCases.setPref(id, level, section),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["chat", "prefs"] }),
  });
  const [naming, setNaming] = useState<string | null>(null); // conversation getting a new section
  const [sectionName, setSectionName] = useState("");
  const known = [...new Set((prefs.data ?? []).map((p) => p.section).filter(Boolean))].sort();
  // S85: my order of sections; drag a header onto another (or ↑) to move it before that one.
  const order = useQuery({ queryKey: ["chat", "sections"], queryFn: chatUseCases.sectionOrder });
  const [dragging, setDragging] = useState<string | null>(null);
  const saveOrder = useMutation({
    mutationFn: chatUseCases.setSectionOrder,
    onSuccess: (o) => qc.setQueryData(["chat", "sections"], o),
  });
  const reorder = (from: string, to: string) => {
    if (from === to) return;
    const current = sections(conversations, prefs.data ?? [], order.data ?? []).map((x) => x.title).filter((t) => known.includes(t));
    saveOrder.mutate(moveSection(current, from, to));
  };
  const item = (c: Conversation) => {
    const p = prefOf(c.id);
    const muted = p.level === "muted";
    return (
      <li key={c.id} className={muted ? "muted-conv" : ""}>
        <Link to={`/chat/${c.id}`} className={`${c.id === current ? "active" : ""} ${c.unread > 0 && !muted ? "unread" : ""}`} aria-current={c.id === current ? "page" : undefined}>
          {title(c)}
          {peer(c)?.emoji && <span title={peer(c)?.text}> {peer(c)?.emoji}</span>}
          {peer(c)?.dnd && <span aria-label={t("Do not disturb")}> 🌙</span>}
          {muted && <span aria-label={t("Muted")}> 🔕</span>}
          {p.level === "all" && <span aria-label={t("All messages")}> 🔔</span>}
          {c.unread > 0 && !muted && <span className="badge" aria-label={t("{n} unread", { n: c.unread })}>{c.unread}</span>}
        </Link>
        <select className="ghost conv-options" aria-label={t("Options for {name}", { name: title(c) })} value="" onChange={(e) => {
          const v = e.target.value;
          if (v.startsWith("level:")) setPref.mutate({ id: c.id, level: v.slice(6) as Level, section: p.section });
          else if (v === "section:new") { setNaming(c.id); setSectionName(""); }
          else if (v.startsWith("section:")) setPref.mutate({ id: c.id, level: p.level, section: v.slice(8) });
        }}>
          <option value="">⋯</option>
          <option value="level:all">{t("🔔 Every message")}</option>
          <option value="level:mentions">{t("@ Mentions only")}</option>
          <option value="level:muted">{t("🔕 Mute")}</option>
          {known.filter((k) => k !== p.section).map((k) => <option key={k} value={`section:${k}`}>{t("Move to {section}", { section: k })}</option>)}
          {p.section && <option value="section:">{t("Move back")}</option>}
          <option value="section:new">{t("New section…")}</option>
        </select>
        {naming === c.id && (
          <form onSubmit={(e) => { e.preventDefault(); setPref.mutate({ id: c.id, level: p.level, section: sectionName }); setNaming(null); }}>
            <input aria-label={t("Section name")} value={sectionName} onChange={(e) => setSectionName(e.target.value)} maxLength={40} autoFocus />
          </form>
        )}
      </li>
    );
  };
  const error = create.error ?? openDM.error;
  const [q, setQ] = useState("");
  const [query, setQuery] = useState("");
  const results = useQuery({ queryKey: ["chat", "search", query], queryFn: () => chatUseCases.search(query), enabled: query.length >= 2 });
  const titleOf = (id: string) => {
    const c = conversations.find((x) => x.id === id);
    return c ? title(c) : "";
  };
  return (
    <nav aria-label={t("Conversations")} className="chat-sidebar panel">
      <MyStatus me={me} />
      <form role="search" onSubmit={(e) => { e.preventDefault(); setQuery(q.trim()); }}>
        <input type="search" aria-label={t("Search messages")} placeholder={t("Search messages")} value={q} onChange={(e) => setQ(e.target.value)} />
      </form>
      {query.length >= 2 && (
        <ul className="chat-results" aria-label={t("Search results")}>
          {results.data?.length === 0 && <li className="muted">{t("No messages found.")}</li>}
          {results.data?.map((m) => (
            <li key={m.id}>
              <Link to={`/chat/${m.conversation_id}`} onClick={() => setQuery("")}>
                <span className="muted">{titleOf(m.conversation_id)} · {names.get(m.author_id) ?? t("Someone")}</span><br />{m.body}
              </Link>
            </li>
          ))}
        </ul>
      )}
      <Link to="/chat" className={`chat-activity-link ${current ? "" : "active"}`} aria-current={current ? undefined : "page"}>{t("🔔 Activity")}</Link>
      {sections(conversations, prefs.data ?? [], order.data ?? []).map((sec, i, all) => (
        <section key={sec.title} aria-label={sec.title}
          onDragOver={(e) => { if (dragging && known.includes(sec.title)) e.preventDefault(); }}
          onDrop={(e) => { e.preventDefault(); if (dragging) reorder(dragging, sec.title); setDragging(null); }}>
          <h2 draggable={known.includes(sec.title)} onDragStart={() => setDragging(sec.title)} onDragEnd={() => setDragging(null)}
            className={known.includes(sec.title) ? "draggable-section" : ""}>
            {sec.title}
            {known.includes(sec.title) && i > 0 && known.includes(all[i - 1]!.title) && (
              <button className="ghost" aria-label={t("Move {section} up", { section: sec.title })} onClick={() => reorder(sec.title, all[i - 1]!.title)}>↑</button>
            )}
          </h2>
          <ul>{sec.items.map(item)}</ul>
        </section>
      ))}
      <div className="row">
        <button className="ghost" onClick={() => setForm(form === "channel" ? null : "channel")}>{t("New channel")}</button>
        <button className="ghost" onClick={() => setForm(form === "dm" ? null : "dm")}>{t("New message")}</button>
      </div>
      {form && (
        <form onSubmit={submit} className="chat-new">
          {form === "channel" ? (
            <>
              <label>{t("Channel name")} <input value={name} onChange={(e) => setName(e.target.value)} placeholder="design" required /></label>
              <label><input type="checkbox" checked={priv} onChange={(e) => setPriv(e.target.checked)} /> {t("Private")}</label>
              <button>{t("Create")}</button>
            </>
          ) : (
            <>
              <fieldset>
                <legend>{t("People")}</legend>
                {[...names].filter(([uid]) => uid !== me).map(([uid, n]) => (
                  <label key={uid}>
                    <input type="checkbox" checked={picked.includes(uid)}
                      onChange={(e) => setPicked((p) => (e.target.checked ? [...p, uid] : p.filter((x) => x !== uid)))} /> {n}
                  </label>
                ))}
              </fieldset>
              <button disabled={picked.length === 0}>{t("Open")}</button>
            </>
          )}
          {error && <p role="alert" className="error">{error.message}</p>}
        </form>
      )}
    </nav>
  );
}

function ConversationView({ id, conversation, title, names, me, typers, onThread, extras }: {
  id: string; conversation?: Conversation; title: string; names: Map<string, string>; me: string; typers: string[];
  onThread: (id: string) => void; extras?: ReactNode;
}) {
  const t = useT();
  const qc = useQueryClient();
  const history = useQuery({ queryKey: historyKey(id), queryFn: () => chatUseCases.history(id) });
  const [showPins, setShowPins] = useState(false);
  const pins = useQuery({ queryKey: ["chat", "pins", id], queryFn: () => chatUseCases.pins(id), enabled: showPins });
  const [older, setOlder] = useState<Message[]>([]);
  const [exhausted, setExhausted] = useState(false);
  useEffect(() => {
    void chatUseCases.markRead(id).then(() => qc.invalidateQueries({ queryKey: ["chat", "conversations"] }), () => {});
  }, [id, qc]);
  const refreshList = () => qc.invalidateQueries({ queryKey: ["chat", "conversations"] });
  const join = useMutation({ mutationFn: () => chatUseCases.join(id), onSuccess: refreshList });
  const post = useMutation({
    mutationFn: (body: string) => chatUseCases.post(id, body),
    onSuccess: (m) => qc.setQueryData<Message[]>(historyKey(id), (list) => applyEvent(list ?? [], id, { type: "message.posted", message: m })),
  });
  const [note, setNote] = useState("");
  const later = useMutation({
    mutationFn: ({ body, at }: { body: string; at: Date }) => chatUseCases.schedule(id, body, at),
    onSuccess: (sc) => {
      setNote(t("🕒 Scheduled for {time}.", { time: new Date(sc.send_at).toLocaleString() }));
      void qc.invalidateQueries({ queryKey: scheduledKey });
    },
  });
  const remind = useMutation({
    mutationFn: ({ text, at }: { text: string; at: Date }) => chatUseCases.remind(id, text, at),
    onSuccess: (r) => setNote(t("⏰ I'll remind you at {time}: {text}", { time: new Date(r.at).toLocaleString(), text: r.text })),
  });
  const commands = useContext(Commands);
  const command = useMutation({ mutationFn: (c: SlashCommand) => commands!(c), onSuccess: setNote, onError: (e) => setNote(`⚠️ ${e.message}`) });
  /** Sends, or for "/remind me …" sets a personal reminder (S67); /issue, /assign and /status run commands (S72). */
  const send = (body: string) => {
    setNote("");
    if (/^\/(issue|assign|status)\b/i.test(body)) {
      const c = parseCommand(body);
      if (c && commands) command.mutate(c);
      else setNote(COMMAND_HELP);
      return;
    }
    if (/^\/remind\b/i.test(body)) {
      const r = parseRemind(body, new Date());
      if (r) remind.mutate(r);
      else setNote(t("Try: /remind me in 20 minutes to …, /remind me tomorrow …, /remind me at 15:30 …"));
      return;
    }
    post.mutate(body);
  };
  const upload = useMutation({
    mutationFn: (f: File) => {
      if (f.size > MAX_FILE) throw new Error(t("{name} is larger than 25 MB", { name: f.name }));
      return chatUseCases.upload(id, f, f.name);
    },
    onSuccess: (m) => qc.setQueryData<Message[]>(historyKey(id), (list) => applyEvent(list ?? [], id, { type: "message.posted", message: m })),
  });
  const replace = (m: Message) => {
    qc.setQueryData<Message[]>(historyKey(id), (list) => list?.map((x) => (x.id === m.id ? m : x)));
    setOlder((list) => list.map((x) => (x.id === m.id ? m : x)));
  };
  const loadOlder = async () => {
    const all = [...(history.data ?? []), ...older];
    const page = await chatUseCases.history(id, all[all.length - 1]?.id);
    setOlder((o) => [...o, ...page]);
    if (page.length < PAGE) setExhausted(true);
  };
  const [showSettings, setShowSettings] = useState(false);
  const messages = [...(history.data ?? []), ...older].reverse(); // oldest at the top
  const receipts = useQuery({ queryKey: ["chat", "receipts", id], queryFn: () => chatUseCases.receipts(id), enabled: conversation?.kind === "dm" });
  const seenId = conversation?.kind === "dm" ? seen(messages, receipts.data ?? [], me) : null;
  const member = conversation?.member ?? true;
  return (
    <section className="chat-main">
      <header className="chat-header">
        <h1>{title}</h1>
        {conversation?.topic && <p className="muted">{conversation.topic}</p>}
        <button className="ghost" aria-expanded={showPins} onClick={() => setShowPins((v) => !v)}>{t("📌 Pinned")}</button>
        {conversation?.kind === "channel" && (
          <button className="ghost" aria-expanded={showSettings} onClick={() => setShowSettings((v) => !v)}>{t("⚙ Settings")}</button>
        )}
      </header>
      {conversation?.archived && <p className="chat-archived" role="status">{t("🗄 This channel is archived — read-only.")}</p>}
      {showSettings && conversation && <ChannelSettings c={conversation} me={me} names={names} />}
      {showPins && (
        <ul className="chat-pins panel" aria-label={t("Pinned messages")}>
          {pins.data?.length === 0 && <li className="muted">{t("Nothing pinned yet.")}</li>}
          {pins.data?.map((p) => <li key={p.id}><b>{names.get(p.author_id) ?? t("Someone")}</b>: {p.body}</li>)}
        </ul>
      )}
      {extras}
      <div role="log" aria-label={t("Messages")} className="chat-log">
        {(history.data?.length ?? 0) >= PAGE && !exhausted && <button className="ghost" onClick={() => void loadOlder()}>{t("Load older")}</button>}
        {history.data?.length === 0 && <p className="muted">{t("No messages yet. Say hello!")}</p>}
        {messages.map((m) => (
          <div key={m.id}>
            <MessageItem m={m} names={names} me={me} canAct={member} onChange={replace} onThread={() => onThread(m.id)} />
            {m.id === seenId && <p className="chat-seen muted" aria-label={t("Seen")}>{t("✓ Seen")}</p>}
          </div>
        ))}
      </div>
      <p className="chat-typing muted" aria-live="polite">{typers.length > 0 ? t(typers.length === 1 ? "{names} is typing…" : "{names} are typing…", { names: typers.join(", ") }) : " "}</p>
      {conversation?.archived ? null : member ? (
        <div className="chat-compose-row">
          <label className="ghost chat-attach" title={t("Attach a file (25 MB at most)")}>
            📎
            <input type="file" hidden aria-label={t("Attach a file")} onChange={(e) => {
              const f = e.target.files?.[0];
              e.target.value = "";
              if (f) upload.mutate(f);
            }} />
          </label>
          <ClipButton onClip={(f) => upload.mutate(f)} />
          <Composer label={t("Message {name}", { name: title })} onSend={send} onSchedule={(body, at) => later.mutate({ body, at })}
            onTyping={() => void chatUseCases.typing(id).catch(() => {})} />
        </div>
      ) : (
        <button onClick={() => join.mutate()}>{t("Join {name}", { name: title })}</button>
      )}
      {post.error && <p role="alert" className="error">{post.error.message}</p>}
      {(later.error ?? remind.error) && <p role="alert" className="error">{(later.error ?? remind.error)!.message}</p>}
      {note && <p role="status" className="muted">{note}</p>}
      {upload.isPending && <p className="muted">{t("Uploading…")}</p>}
      {upload.error && <p role="alert" className="error">{upload.error.message}</p>}
    </section>
  );
}

function MessageItem({ m, names, me, canAct, onChange, onThread }: {
  m: Message; names: Map<string, string>; me: string; canAct: boolean; onChange: (m: Message) => void; onThread?: () => void;
}) {
  const t = useT();
  const [editing, setEditing] = useState(false);
  const react = useMutation({ mutationFn: (emoji: string) => chatUseCases.react(m.id, emoji), onSuccess: onChange });
  const edit = useMutation({
    mutationFn: (body: string) => chatUseCases.edit(m.id, body),
    onSuccess: (x) => {
      setEditing(false);
      onChange(x);
    },
  });
  const pin = useMutation({ mutationFn: () => (m.pinned_by ? chatUseCases.unpin(m.id) : chatUseCases.pin(m.id)), onSuccess: onChange });
  const remove = useMutation({ mutationFn: () => chatUseCases.remove(m.id), onSuccess: () => onChange({ ...m, deleted: true, body: "", reactions: [], issues: [] }) });
  const mine = m.author_id === me && !m.kind;
  if (m.kind === "webhook") {
    return (
      <article className="chat-message chat-system" aria-label={t("Webhook at {time}", { time: time(m.created_at) })}>
        <p className="chat-body">🔌 {m.body} <time className="muted" dateTime={m.created_at}>{time(m.created_at)}</time></p>
      </article>
    );
  }
  if (m.kind === "workflow") {
    return (
      <article className="chat-message chat-system" aria-label={t("Workflow at {time}", { time: time(m.created_at) })}>
        <p className="chat-body">🤖 {m.body} <time className="muted" dateTime={m.created_at}>{time(m.created_at)}</time></p>
      </article>
    );
  }
  if (m.kind === "transcript" || m.kind === "summary") {
    return (
      <article className="chat-message chat-system" aria-label={t(m.kind === "summary" ? "Summary at {time}" : "Transcript at {time}", { time: time(m.created_at) })}>
        <p className="chat-body chat-transcript">{m.body}</p>
      </article>
    );
  }
  if (m.kind === "project_event") {
    return (
      <article className="chat-message chat-system" aria-label={t("Project news at {time}", { time: time(m.created_at) })}>
        <p className="muted">📋 <Who id={m.author_id} names={names} /> {m.body} <time dateTime={m.created_at}>{time(m.created_at)}</time></p>
        <IssuePreviews m={m} />
      </article>
    );
  }
  if (m.kind) {
    return (
      <article className="chat-message chat-system" aria-label={t("Huddle at {time}", { time: time(m.created_at) })}>
        <p className="muted">
          🎧 {m.kind === "huddle_recording" ? <><Who id={m.author_id} names={names} /> {t("recorded the huddle")}</>
            : m.kind === "huddle_started" ? <><Who id={m.author_id} names={names} /> {m.body}</> : m.body}{" "}
          <time dateTime={m.created_at}>{time(m.created_at)}</time>
        </p>
        {m.kind === "huddle_recording" && m.file && !m.deleted && <Attachment file={m.file} />}
      </article>
    );
  }
  return (
    <article className="chat-message" aria-label={t("{name} at {time}", { name: names.get(m.author_id) ?? t("Someone"), time: time(m.created_at) })}>
      <header>
        <Who id={m.author_id} names={names} /> <time className="muted" dateTime={m.created_at}>{time(m.created_at)}</time>
        {m.edited_at && !m.deleted && <span className="muted">{t(" (edited)")}</span>}
        {m.pinned_by && <span className="muted" aria-label={t("pinned")}> 📌</span>}
      </header>
      {m.deleted ? (
        <p className="muted"><i>{t("This message was deleted.")}</i></p>
      ) : editing ? (
        <Composer label={t("Edit message")} initial={m.body} onSend={(b) => edit.mutate(b)} onCancel={() => setEditing(false)} />
      ) : (
        <p className="chat-body">
          {splitMentions(m.body).map((s, i) => (s.kind === "text" ? <span key={i}>{s.text}</span> : <span key={i} className={`chat-${s.kind}`}>{s.text}</span>))}
        </p>
      )}
      {m.file && !m.deleted && <Attachment file={m.file} />}
      <IssuePreviews m={m} />
      {!m.deleted && (
        <div className="chat-actions">
          {m.reactions.map((r) => (
            <button key={r.emoji} className={`reaction ${r.user_ids.includes(me) ? "mine" : ""}`} disabled={!canAct}
              aria-pressed={r.user_ids.includes(me)} onClick={() => react.mutate(r.emoji)}>
              {r.emoji} {r.user_ids.length}
            </button>
          ))}
          {canAct && QUICK.filter((q) => !m.reactions.some((r) => r.emoji === q)).map((q) => (
            <button key={q} className="ghost quick" aria-label={t("React {emoji}", { emoji: q })} onClick={() => react.mutate(q)}>{q}</button>
          ))}
          {onThread && (
            <button className="ghost" onClick={onThread}>
              {m.reply_count > 0 ? t(m.reply_count === 1 ? "{n} reply" : "{n} replies", { n: m.reply_count }) : t("Reply in thread")}
            </button>
          )}
          {canAct && <button className="ghost" onClick={() => pin.mutate()}>{m.pinned_by ? t("Unpin") : t("Pin")}</button>}
          <SaveMenu m={m} />
          {canAct && !m.thread_id && <IssueFromMessage m={m} onDone={onChange} />}
          {mine && canAct && <button className="ghost" onClick={() => setEditing(true)}>{t("Edit")}</button>}
          {mine && canAct && <button className="ghost" onClick={() => remove.mutate()}>{t("Delete")}</button>}
        </div>
      )}
    </article>
  );
}

function Attachment({ file }: { file: NonNullable<Message["file"]> }) {
  const label = `${file.name} (${fileSize(file.size)})`;
  if (file.type.startsWith("image/") && file.type !== "image/svg+xml") {
    return <a href={file.url} target="_blank" rel="noreferrer"><img className="chat-image" src={file.url} alt={file.name} /></a>;
  }
  if (file.type.startsWith("video/")) return <video className="chat-video" src={file.url} controls aria-label={label} />;
  if (file.type.startsWith("audio/")) return <audio src={file.url} controls aria-label={label} />;
  return <a className="chat-file" href={file.url} download={file.name}>📎 {label}</a>;
}

export function ThreadPanel({ parent, names, me, onClose }: { parent: string; names: Map<string, string>; me: string; onClose: () => void }) {
  const t = useT();
  const qc = useQueryClient();
  const replies = useQuery({ queryKey: threadKey(parent), queryFn: () => chatUseCases.replies(parent) });
  const refresh = () => qc.invalidateQueries({ queryKey: threadKey(parent) });
  const reply = useMutation({
    mutationFn: (body: string) => chatUseCases.reply(parent, body),
    onSuccess: (m) => qc.setQueryData<Message[]>(threadKey(parent), (list) => (list?.some((x) => x.id === m.id) ? list : [...(list ?? []), m])),
  });
  return (
    <aside aria-label={t("Thread")} className="chat-thread panel">
      <header className="chat-thread-head">
        <h2>{t("Thread")}</h2>
        <button className="ghost" aria-label={t("Close thread")} onClick={onClose}>✕</button>
      </header>
      {replies.data?.map((m) => <MessageItem key={m.id} m={m} names={names} me={me} canAct onChange={() => void refresh()} />)}
      <Composer label={t("Reply")} onSend={(b) => reply.mutate(b)} />
      {reply.error && <p role="alert" className="error">{reply.error.message}</p>}
    </aside>
  );
}

/** A message box: Enter sends, Shift+Enter breaks the line; typing is announced at most every 3 s. */
function Composer({ label, initial = "", onSend, onSchedule, onTyping, onCancel }: {
  label: string; initial?: string; onSend: (body: string) => void; onTyping?: () => void; onCancel?: () => void;
  /** Send later (S67): offers times from now. */
  onSchedule?: (body: string, at: Date) => void;
}) {
  const t = useT();
  const [text, setText] = useState(initial);
  const lastTyping = useRef(0);
  const send = () => {
    const body = text.trim();
    if (!body) return;
    onSend(body);
    setText("");
  };
  const onKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      send();
    } else if (e.key === "Escape") onCancel?.();
  };
  return (
    <div className="chat-composer">
      <textarea aria-label={label} rows={2} value={text} placeholder={label} onKeyDown={onKeyDown}
        onChange={(e) => {
          setText(e.target.value);
          if (onTyping && Date.now() - lastTyping.current > 3000) {
            lastTyping.current = Date.now();
            onTyping();
          }
        }} />
      <button onClick={send} disabled={!text.trim()}>{t("Send")}</button>
      {onSchedule && (
        <select aria-label={t("Send later")} value="" disabled={!text.trim()} onChange={(e) => {
          const p = remindPresets(new Date())[Number(e.target.value)];
          if (!p || !text.trim()) return;
          onSchedule(text.trim(), p.at);
          setText("");
        }}>
          <option value="">🕒</option>
          {remindPresets(new Date()).map((p, i) => <option key={p.label} value={i}>{t("Send {when}", { when: p.label.toLowerCase() })}</option>)}
        </select>
      )}
    </div>
  );
}
