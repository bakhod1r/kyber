// whiteboard: HTTP gateway and realtime events (infrastructure).
import { request, seg, type List } from "../../../shared/http";
import { subscribe } from "../../../shared/stream";
import type { Element, Op, OpResult, Whiteboard } from "../domain/model";

export const whiteboardGateway = {
  list: (conversationId: string) =>
    request<List<Whiteboard>>("GET", `/api/v1/conversations/${seg(conversationId)}/whiteboards`).then((r) => r.items),
  create: (conversationId: string, title: string) => request<Whiteboard>("POST", `/api/v1/conversations/${seg(conversationId)}/whiteboards`, { title }),
  get: (id: string) => request<Whiteboard>("GET", `/api/v1/whiteboards/${seg(id)}`),
  apply: (id: string, ops: Op[]) => request<{ results: OpResult[] }>("POST", `/api/v1/whiteboards/${seg(id)}/ops`, { ops }).then((r) => r.results),
  cursor: (id: string, x: number, y: number) => request<undefined>("POST", `/api/v1/whiteboards/${seg(id)}/cursor`, { x, y }),
};

export type OpsEvent = { whiteboard_id: string; by: string; elements: Element[] };
export type CursorEvent = { whiteboard_id: string; user_id: string; x: number; y: number };

export function onWhiteboardEvents(onOps: (e: OpsEvent) => void, onCursor: (e: CursorEvent) => void): () => void {
  return subscribe(["whiteboard.ops", "whiteboard.cursor"], (type, data) => {
    if (type === "whiteboard.ops") onOps(data as OpsEvent);
    else onCursor(data as CursorEvent);
  });
}
