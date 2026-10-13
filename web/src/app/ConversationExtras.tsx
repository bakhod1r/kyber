import { useQueries, useQuery } from "@tanstack/react-query";
import { type Presentable, type Presentation } from "../contexts/call/application";
import { Huddle } from "../contexts/call/ui/Huddle";
import { docsUseCases } from "../contexts/docs/application";
import { DocPage } from "../contexts/docs/ui/DocPage";
import { projectUseCases } from "../contexts/project/application";
import { whiteboardUseCases } from "../contexts/whiteboard/application";
import { WhiteboardBoard, Whiteboards } from "../contexts/whiteboard/ui/Whiteboards";

/**
 * The huddle and whiteboards of the open conversation, composed by the app shell: the huddle can present
 * this conversation's whiteboards and the docs of my projects to everyone (S60).
 */
export function ConversationExtras({ conversationId, me, names }: { conversationId: string; me: string; names: Map<string, string> }) {
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
      <Huddle conversationId={conversationId} me={me} names={names} presentables={presentables} renderPresentation={render} />
      <Whiteboards conversationId={conversationId} me={me} names={names} />
    </div>
  );
}
