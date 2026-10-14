// whiteboard: HTTP gateway and realtime events (infrastructure).
import { ApiError, isProblem, request, seg, type List } from "../../../shared/http";
import { subscribe } from "../../../shared/stream";
import type { Element, ElementComment, Op, OpResult, Whiteboard } from "../domain/model";

export const whiteboardGateway = {
  list: (conversationId: string) =>
    request<List<Whiteboard>>("GET", `/api/v1/conversations/${seg(conversationId)}/whiteboards`).then((r) => r.items),
  create: (conversationId: string, title: string) => request<Whiteboard>("POST", `/api/v1/conversations/${seg(conversationId)}/whiteboards`, { title }),
  get: (id: string) => request<Whiteboard>("GET", `/api/v1/whiteboards/${seg(id)}`),
  /** S96: uploads a picture (raw body; the header satisfies the CSRF guard for cookie sessions). */
  uploadImage: async (id: string, file: Blob): Promise<{ id: string; url: string }> => {
    const res = await fetch(`/api/v1/whiteboards/${seg(id)}/images`, {
      method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/octet-stream", "X-Kyber-Request": "1" }, body: file,
    });
    const data: unknown = await res.json().catch(() => undefined);
    if (!res.ok) throw new ApiError(res.status, isProblem(data) ? data.detail || data.title : `upload failed (${res.status})`, isProblem(data) ? data.code : "");
    return data as { id: string; url: string };
  },
  apply: (id: string, ops: Op[]) => request<{ results: OpResult[] }>("POST", `/api/v1/whiteboards/${seg(id)}/ops`, { ops }).then((r) => r.results),
  comments: (id: string) => request<List<ElementComment>>("GET", `/api/v1/whiteboards/${seg(id)}/comments`).then((r) => r.items),
  comment: (id: string, elementId: string, body: string) =>
    request<ElementComment>("POST", `/api/v1/whiteboards/${seg(id)}/comments`, { element_id: elementId, body }),
  removeComment: (id: string) => request<undefined>("DELETE", `/api/v1/whiteboard-comments/${seg(id)}`),
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

/** whiteboard.comment: a comment added or removed on a board (S86). */
export function onWhiteboardComments(fn: (boardId: string) => void): () => void {
  return subscribe(["whiteboard.comment"], (_type, data) => fn((data as { whiteboard_id: string }).whiteboard_id));
}
