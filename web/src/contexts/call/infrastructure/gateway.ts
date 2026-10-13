// call: HTTP gateway to the huddle API and its realtime events (infrastructure).
import { request, seg } from "../../../shared/http";
import { onConnect, subscribe } from "../../../shared/stream";

export { onConnect };
import type { Call, FollowView, IceServer, Media, PresentKind, Signal, SignalKind } from "../domain/model";

const conv = (id: string) => `/api/v1/conversations/${seg(id)}/call`;
const call = (id: string) => `/api/v1/calls/${seg(id)}`;

export const callGateway = {
  live: (conversationId: string) => request<{ call: Call | null }>("GET", conv(conversationId)).then((r) => r.call),
  join: (conversationId: string) => request<Call>("POST", conv(conversationId)),
  leave: (callId: string) => request<undefined>("POST", `${call(callId)}/leave`),
  media: (callId: string, m: Media) => request<Call>("PUT", `${call(callId)}/media`, m),
  signal: (callId: string, to: string, kind: SignalKind, data: string) =>
    request<undefined>("POST", `${call(callId)}/signal`, { to, kind, data }),
  present: (callId: string, kind: PresentKind, ref: string) => request<Call>("PUT", `${call(callId)}/presentation`, { kind, ref }),
  stopPresenting: (callId: string) => request<Call>("DELETE", `${call(callId)}/presentation`),
  follow: (callId: string, v: { x: number; y: number; zoom: number }) => request<undefined>("POST", `${call(callId)}/presentation/follow`, v),
  startRecording: (callId: string) => request<Call>("PUT", `${call(callId)}/recording`),
  stopRecording: (callId: string) => request<Call>("DELETE", `${call(callId)}/recording`),
  /** Posts the finished recording to the conversation (raw body; the header satisfies the CSRF guard). */
  uploadRecording: async (conversationId: string, video: Blob, name: string): Promise<void> => {
    const res = await fetch(`/api/v1/conversations/${seg(conversationId)}/files?recording=1&name=${encodeURIComponent(name)}`, {
      method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/octet-stream", "X-Kyber-Upload": "1" }, body: video,
    });
    if (!res.ok) throw new Error(`Uploading the recording failed (${res.status})`);
  },
  ice: () => request<{ ice_servers: IceServer[] }>("GET", "/api/v1/calls/ice").then((r) => r.ice_servers),
};

/** call.updated, call.signal and call.follow from the shared realtime stream. */
export function onCallEvents(onCall: (c: Call) => void, onSignal: (s: Signal) => void, onFollow: (v: FollowView) => void = () => {}): () => void {
  return subscribe(["call.updated", "call.signal", "call.follow"], (type, data) => {
    if (type === "call.updated") onCall(data as Call);
    else if (type === "call.signal") onSignal(data as Signal);
    else onFollow(data as FollowView);
  });
}
