import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { type Presentable, type Presentation } from "../contexts/call/application";
import { Huddle } from "../contexts/call/ui/Huddle";
import { notifyUseCases } from "../contexts/notify/application";
import { docsUseCases } from "../contexts/docs/application";
import { DocPage } from "../contexts/docs/ui/DocPage";
import { projectUseCases } from "../contexts/project/application";
import { whiteboardUseCases } from "../contexts/whiteboard/application";
import { WhiteboardBoard, Whiteboards } from "../contexts/whiteboard/ui/Whiteboards";
import { type Call } from "../contexts/call/application";
import { useT } from "../shared/i18n/i18n";
import { chatUseCases } from "../contexts/chat/application";
import { ThreadPanel } from "../contexts/chat/ui/ChatPage";

/** S79: docs and links kept in the conversation's header; a doc opens right here. */
function Bookmarks({ conversationId, docs }: { conversationId: string; docs: Presentable[] }) {
  const t = useT();
  const qc = useQueryClient();
  const key = ["chat", "bookmarks", conversationId];
  const list = useQuery({ queryKey: key, queryFn: () => chatUseCases.bookmarks(conversationId) });
  const done = () => qc.invalidateQueries({ queryKey: key });
  const add = useMutation({ mutationFn: (b: { kind: "doc" | "link"; ref: string; title: string }) => chatUseCases.addBookmark(conversationId, b), onSuccess: done });
  const remove = useMutation({ mutationFn: (id: string) => chatUseCases.removeBookmark(id), onSuccess: done });
  const [open, setOpen] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const [url, setUrl] = useState("");
  const [title, setTitle] = useState("");
  return (
    <div className="chat-bookmarks">
      <ul aria-label={t("Bookmarks")} className="row">
        {(list.data ?? []).map((b) => (
          <li key={b.id}>
            {b.kind === "doc"
              ? <button className="ghost" aria-pressed={open === b.ref} onClick={() => setOpen(open === b.ref ? null : b.ref)}>📄 {b.title}</button>
              : <a href={b.ref} target="_blank" rel="noopener noreferrer">🔗 {b.title}</a>}
            <button className="ghost" aria-label={t("Remove {name}", { name: b.title })} onClick={() => remove.mutate(b.id)}>✕</button>
          </li>
        ))}
        <li><button className="ghost" onClick={() => setAdding((v) => !v)}>+ {t("Bookmark")}</button></li>
      </ul>
      {adding && (
        <div className="row chat-new" role="group" aria-label={t("Add a bookmark")}>
          {docs.length > 0 && (
            <select aria-label={t("Add a doc")} value="" onChange={(e) => {
              const d = docs.find((x) => x.ref === e.target.value);
              if (d) add.mutate({ kind: "doc", ref: d.ref, title: d.title }, { onSuccess: () => setAdding(false) });
            }}>
              <option value="">📄 {t("Add a doc…")}</option>
              {docs.map((d) => <option key={d.ref} value={d.ref}>{d.title}</option>)}
            </select>
          )}
          <form className="row" onSubmit={(e) => { e.preventDefault(); add.mutate({ kind: "link", ref: url, title }, { onSuccess: () => { setAdding(false); setUrl(""); setTitle(""); } }); }}>
            <input aria-label={t("Link")} placeholder="https://…" value={url} onChange={(e) => setUrl(e.target.value)} />
            <input aria-label={t("Link title")} placeholder={t("Title")} value={title} onChange={(e) => setTitle(e.target.value)} />
            <button>{t("Add link")}</button>
          </form>
          {add.error && <span role="alert" className="error">{add.error.message}</span>}
        </div>
      )}
      {open && <DocPage id={open} />}
    </div>
  );
}

/** The huddle's chat (S75): the thread of the conversation's "started a huddle" message for this call. */
function HuddleChat({ call, me, names }: { call: Call; me: string; names: Map<string, string> }) {
  const t = useT();
  const started = useQuery({
    queryKey: ["chat", "huddle-start", call.id],
    queryFn: async () => {
      const at = Date.parse(call.started_at);
      const page = await chatUseCases.history(call.conversation_id);
      return page.find((m) => m.kind === "huddle_started" && Math.abs(Date.parse(m.created_at) - at) < 60_000)?.id ?? null;
    },
    refetchInterval: (q) => (q.state.data ? false : 2000), // the system message is written by the relay
  });
  if (!started.data) return <p className="muted">{t("Opening the huddle chat…")}</p>;
  return <ThreadPanel parent={started.data} names={names} me={me} onClose={() => {}} />;
}

/**
 * The huddle and whiteboards of the open conversation, composed by the app shell: the huddle can present
 * this conversation's whiteboards and the docs of my projects to everyone (S60).
 */
export function ConversationExtras({ conversationId, me, names }: { conversationId: string; me: string; names: Map<string, string> }) {
  const qc = useQueryClient();
  const [params] = useSearchParams(); // ?huddle=join from a meeting (S76)
  // Opening the conversation reads my mentions in it (S64).
  useEffect(() => {
    void notifyUseCases.markConversationRead(conversationId).then(() => qc.invalidateQueries({ queryKey: ["notifications"] }), () => {});
  }, [conversationId, qc]);
  const boards = useQuery({ queryKey: ["whiteboards", conversationId], queryFn: () => whiteboardUseCases.list(conversationId) });
  const projects = useQuery({ queryKey: ["projects"], queryFn: projectUseCases.projects });
  const docs = useQueries({ queries: (projects.data ?? []).map((p) => ({ queryKey: ["docs", p.key], queryFn: () => docsUseCases.pages(p.key) })) });
  const presentables: Presentable[] = [
    ...(boards.data ?? []).map((b) => ({ kind: "whiteboard" as const, ref: b.id, title: b.title })),
    ...docs.flatMap((q) => q.data ?? []).map((d) => ({ kind: "doc" as const, ref: d.id, title: `${d.project_key}: ${d.title}` })),
  ];
  const render = (p: Presentation) =>
    p.kind === "whiteboard" ? <WhiteboardBoard id={p.ref} me={me} names={names} /> : <DocPage id={p.ref} />;
  return (
    <div className="conversation-extras">
      <Bookmarks conversationId={conversationId} docs={presentables.filter((p) => p.kind === "doc")} />
      <Huddle conversationId={conversationId} me={me} names={names} presentables={presentables} renderPresentation={render}
        autoJoin={params.get("huddle") === "join"}
        renderChat={(call) => <HuddleChat call={call} me={me} names={names} />} />
      <Whiteboards conversationId={conversationId} me={me} names={names} />
    </div>
  );
}
