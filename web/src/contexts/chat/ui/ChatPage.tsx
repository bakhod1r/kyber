import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, type KeyboardEvent, type ReactNode, useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { identityUseCases } from "../../identity/application";
import { projectUseCases } from "../../project/application";
import {
  applyEvent, type ChatEvent, chatUseCases, type Conversation, conversationTitle, fileSize, MAX_FILE, type Message, splitMentions, useChatStream,
} from "../application";

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
  const names = new Map<string, string>();
  for (const q of lists) for (const m of q.data ?? []) names.set(m.user_id, m.name);
  return names;
}

/** A slot the app shell fills with other contexts' UIs for the open conversation (huddle, whiteboards). */
export type ConversationSlot = (conversationId: string, me: string, names: Map<string, string>) => ReactNode;

/** Team chat (S53, ADR-0009): channels and DMs on the left, the open conversation and its threads on the right. */
export function ChatPage({ extras }: { extras?: ConversationSlot }) {
  const { id } = useParams();
  const qc = useQueryClient();
  const me = useQuery({ queryKey: ["me"], queryFn: identityUseCases.me });
  const names = usePeople();
  const conversations = useQuery({ queryKey: ["chat", "conversations"], queryFn: chatUseCases.conversations });
  const [typing, setTyping] = useState<Record<string, { user: string; until: number }[]>>({});
  const [thread, setThread] = useState<string | null>(null);

  useChatStream((e: ChatEvent) => {
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
  const typers = (typing[id ?? ""] ?? []).filter((x) => x.until > Date.now() && x.user !== myId).map((x) => names.get(x.user) ?? "Someone");

  return (
    <div className="chat">
      <Sidebar conversations={conversations.data ?? []} current={id} title={title} names={names} me={myId} />
      {id ? (
        <ConversationView key={id} id={id} conversation={current} title={current ? title(current) : ""} names={names} me={myId}
          typers={typers} onThread={setThread} extras={current?.member && myId ? extras?.(id, myId, names) : undefined} />
      ) : (
        <section className="chat-empty muted">Pick a channel or start a direct message.</section>
      )}
      {thread && <ThreadPanel parent={thread} names={names} me={myId} onClose={() => setThread(null)} />}
    </div>
  );
}

function Sidebar({ conversations, current, title, names, me }: {
  conversations: Conversation[]; current?: string; title: (c: Conversation) => string; names: Map<string, string>; me: string;
}) {
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
  const item = (c: Conversation) => (
    <li key={c.id}>
      <Link to={`/chat/${c.id}`} className={`${c.id === current ? "active" : ""} ${c.unread > 0 ? "unread" : ""}`} aria-current={c.id === current ? "page" : undefined}>
        {title(c)}
        {c.unread > 0 && <span className="badge" aria-label={`${c.unread} unread`}>{c.unread}</span>}
      </Link>
    </li>
  );
  const error = create.error ?? openDM.error;
  const [q, setQ] = useState("");
  const [query, setQuery] = useState("");
  const results = useQuery({ queryKey: ["chat", "search", query], queryFn: () => chatUseCases.search(query), enabled: query.length >= 2 });
  const titleOf = (id: string) => {
    const c = conversations.find((x) => x.id === id);
    return c ? title(c) : "";
  };
  return (
    <nav aria-label="Conversations" className="chat-sidebar panel">
      <form role="search" onSubmit={(e) => { e.preventDefault(); setQuery(q.trim()); }}>
        <input type="search" aria-label="Search messages" placeholder="Search messages" value={q} onChange={(e) => setQ(e.target.value)} />
      </form>
      {query.length >= 2 && (
        <ul className="chat-results" aria-label="Search results">
          {results.data?.length === 0 && <li className="muted">No messages found.</li>}
          {results.data?.map((m) => (
            <li key={m.id}>
              <Link to={`/chat/${m.conversation_id}`} onClick={() => setQuery("")}>
                <span className="muted">{titleOf(m.conversation_id)} · {names.get(m.author_id) ?? "Someone"}</span><br />{m.body}
              </Link>
            </li>
          ))}
        </ul>
      )}
      <h2>Channels</h2>
      <ul>{conversations.filter((c) => c.kind === "channel").map(item)}</ul>
      <h2>Direct messages</h2>
      <ul>{conversations.filter((c) => c.kind === "dm").map(item)}</ul>
      <div className="row">
        <button className="ghost" onClick={() => setForm(form === "channel" ? null : "channel")}>New channel</button>
        <button className="ghost" onClick={() => setForm(form === "dm" ? null : "dm")}>New message</button>
      </div>
      {form && (
        <form onSubmit={submit} className="chat-new">
          {form === "channel" ? (
            <>
              <label>Channel name <input value={name} onChange={(e) => setName(e.target.value)} placeholder="design" required /></label>
              <label><input type="checkbox" checked={priv} onChange={(e) => setPriv(e.target.checked)} /> Private</label>
              <button>Create</button>
            </>
          ) : (
            <>
              <fieldset>
                <legend>People</legend>
                {[...names].filter(([uid]) => uid !== me).map(([uid, n]) => (
                  <label key={uid}>
                    <input type="checkbox" checked={picked.includes(uid)}
                      onChange={(e) => setPicked((p) => (e.target.checked ? [...p, uid] : p.filter((x) => x !== uid)))} /> {n}
                  </label>
                ))}
              </fieldset>
              <button disabled={picked.length === 0}>Open</button>
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
  const upload = useMutation({
    mutationFn: (f: File) => {
      if (f.size > MAX_FILE) throw new Error(`${f.name} is larger than 25 MB`);
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
  const messages = [...(history.data ?? []), ...older].reverse(); // oldest at the top
  const member = conversation?.member ?? true;
  return (
    <section className="chat-main">
      <header className="chat-header">
        <h1>{title}</h1>
        {conversation?.topic && <p className="muted">{conversation.topic}</p>}
        <button className="ghost" aria-expanded={showPins} onClick={() => setShowPins((v) => !v)}>📌 Pinned</button>
      </header>
      {showPins && (
        <ul className="chat-pins panel" aria-label="Pinned messages">
          {pins.data?.length === 0 && <li className="muted">Nothing pinned yet.</li>}
          {pins.data?.map((p) => <li key={p.id}><b>{names.get(p.author_id) ?? "Someone"}</b>: {p.body}</li>)}
        </ul>
      )}
      {extras}
      <div role="log" aria-label="Messages" className="chat-log">
        {(history.data?.length ?? 0) >= PAGE && !exhausted && <button className="ghost" onClick={() => void loadOlder()}>Load older</button>}
        {history.data?.length === 0 && <p className="muted">No messages yet. Say hello!</p>}
        {messages.map((m) => (
          <MessageItem key={m.id} m={m} names={names} me={me} canAct={member} onChange={replace} onThread={() => onThread(m.id)} />
        ))}
      </div>
      <p className="chat-typing muted" aria-live="polite">{typers.length > 0 ? `${typers.join(", ")} ${typers.length === 1 ? "is" : "are"} typing…` : " "}</p>
      {member ? (
        <div className="chat-compose-row">
          <label className="ghost chat-attach" title="Attach a file (25 MB at most)">
            📎
            <input type="file" hidden aria-label="Attach a file" onChange={(e) => {
              const f = e.target.files?.[0];
              e.target.value = "";
              if (f) upload.mutate(f);
            }} />
          </label>
          <Composer label={`Message ${title}`} onSend={(b) => post.mutate(b)} onTyping={() => void chatUseCases.typing(id).catch(() => {})} />
        </div>
      ) : (
        <button onClick={() => join.mutate()}>Join {title}</button>
      )}
      {post.error && <p role="alert" className="error">{post.error.message}</p>}
      {upload.isPending && <p className="muted">Uploading…</p>}
      {upload.error && <p role="alert" className="error">{upload.error.message}</p>}
    </section>
  );
}

function MessageItem({ m, names, me, canAct, onChange, onThread }: {
  m: Message; names: Map<string, string>; me: string; canAct: boolean; onChange: (m: Message) => void; onThread?: () => void;
}) {
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
  if (m.kind) {
    return (
      <article className="chat-message chat-system" aria-label={`Huddle at ${time(m.created_at)}`}>
        <p className="muted">
          🎧 {m.kind === "huddle_recording" ? <><b>{names.get(m.author_id) ?? "Someone"}</b> recorded the huddle</>
            : m.kind === "huddle_started" ? <><b>{names.get(m.author_id) ?? "Someone"}</b> {m.body}</> : m.body}{" "}
          <time dateTime={m.created_at}>{time(m.created_at)}</time>
        </p>
        {m.kind === "huddle_recording" && m.file && !m.deleted && <Attachment file={m.file} />}
      </article>
    );
  }
  return (
    <article className="chat-message" aria-label={`${names.get(m.author_id) ?? "Someone"} at ${time(m.created_at)}`}>
      <header>
        <b>{names.get(m.author_id) ?? "Someone"}</b> <time className="muted" dateTime={m.created_at}>{time(m.created_at)}</time>
        {m.edited_at && !m.deleted && <span className="muted"> (edited)</span>}
        {m.pinned_by && <span className="muted" aria-label="pinned"> 📌</span>}
      </header>
      {m.deleted ? (
        <p className="muted"><i>This message was deleted.</i></p>
      ) : editing ? (
        <Composer label="Edit message" initial={m.body} onSend={(b) => edit.mutate(b)} onCancel={() => setEditing(false)} />
      ) : (
        <p className="chat-body">
          {splitMentions(m.body).map((s, i) => (s.kind === "text" ? <span key={i}>{s.text}</span> : <span key={i} className={`chat-${s.kind}`}>{s.text}</span>))}
        </p>
      )}
      {m.file && !m.deleted && <Attachment file={m.file} />}
      {m.issues.length > 0 && (
        <ul className="chat-issues">
          {m.issues.map((p) => (
            <li key={p.key}>
              <Link to={`/projects/${p.key.split("-")[0]}?issue=${p.key}`} aria-label={`${p.key} ${p.title}`}>
                <b>{p.key}</b> {p.title} <span className="muted">{p.status}</span>
              </Link>
            </li>
          ))}
        </ul>
      )}
      {!m.deleted && (
        <div className="chat-actions">
          {m.reactions.map((r) => (
            <button key={r.emoji} className={`reaction ${r.user_ids.includes(me) ? "mine" : ""}`} disabled={!canAct}
              aria-pressed={r.user_ids.includes(me)} onClick={() => react.mutate(r.emoji)}>
              {r.emoji} {r.user_ids.length}
            </button>
          ))}
          {canAct && QUICK.filter((q) => !m.reactions.some((r) => r.emoji === q)).map((q) => (
            <button key={q} className="ghost quick" aria-label={`React ${q}`} onClick={() => react.mutate(q)}>{q}</button>
          ))}
          {onThread && (
            <button className="ghost" onClick={onThread}>
              {m.reply_count > 0 ? `${m.reply_count} ${m.reply_count === 1 ? "reply" : "replies"}` : "Reply in thread"}
            </button>
          )}
          {canAct && <button className="ghost" onClick={() => pin.mutate()}>{m.pinned_by ? "Unpin" : "Pin"}</button>}
          {mine && canAct && <button className="ghost" onClick={() => setEditing(true)}>Edit</button>}
          {mine && canAct && <button className="ghost" onClick={() => remove.mutate()}>Delete</button>}
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

function ThreadPanel({ parent, names, me, onClose }: { parent: string; names: Map<string, string>; me: string; onClose: () => void }) {
  const qc = useQueryClient();
  const replies = useQuery({ queryKey: threadKey(parent), queryFn: () => chatUseCases.replies(parent) });
  const refresh = () => qc.invalidateQueries({ queryKey: threadKey(parent) });
  const reply = useMutation({
    mutationFn: (body: string) => chatUseCases.reply(parent, body),
    onSuccess: (m) => qc.setQueryData<Message[]>(threadKey(parent), (list) => (list?.some((x) => x.id === m.id) ? list : [...(list ?? []), m])),
  });
  return (
    <aside aria-label="Thread" className="chat-thread panel">
      <header className="chat-thread-head">
        <h2>Thread</h2>
        <button className="ghost" aria-label="Close thread" onClick={onClose}>✕</button>
      </header>
      {replies.data?.map((m) => <MessageItem key={m.id} m={m} names={names} me={me} canAct onChange={() => void refresh()} />)}
      <Composer label="Reply" onSend={(b) => reply.mutate(b)} />
      {reply.error && <p role="alert" className="error">{reply.error.message}</p>}
    </aside>
  );
}

/** A message box: Enter sends, Shift+Enter breaks the line; typing is announced at most every 3 s. */
function Composer({ label, initial = "", onSend, onTyping, onCancel }: {
  label: string; initial?: string; onSend: (body: string) => void; onTyping?: () => void; onCancel?: () => void;
}) {
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
      <button onClick={send} disabled={!text.trim()}>Send</button>
    </div>
  );
}
