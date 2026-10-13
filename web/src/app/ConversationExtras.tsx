import { useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
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
import { chatUseCases } from "../contexts/chat/application";
import { ThreadPanel } from "../contexts/chat/ui/ChatPage";

/** The huddle's chat (S75): the thread of the conversation's "started a huddle" message for this call. */
function HuddleChat({ call, me, names }: { call: Call; me: string; names: Map<string, string> }) {
  const started = useQuery({
    queryKey: ["chat", "huddle-start", call.id],
    queryFn: async () => {
      const at = Date.parse(call.started_at);
      const page = await chatUseCases.history(call.conversation_id);
      return page.find((m) => m.kind === "huddle_started" && Math.abs(Date.parse(m.created_at) - at) < 60_000)?.id ?? null;
    },
    refetchInterval: (q) => (q.state.data ? false : 2000), // the system message is written by the relay
  });
  if (!started.data) return <p className="muted">Opening the huddle chat…</p>;
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
      <Huddle conversationId={conversationId} me={me} names={names} presentables={presentables} renderPresentation={render}
        autoJoin={params.get("huddle") === "join"}
        renderChat={(call) => <HuddleChat call={call} me={me} names={names} />} />
      <Whiteboards conversationId={conversationId} me={me} names={names} />
    </div>
  );
}
