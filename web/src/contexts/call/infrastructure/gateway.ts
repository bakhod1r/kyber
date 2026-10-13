// call: HTTP gateway to the huddle API and its realtime events (infrastructure).
import { request, seg } from "../../../shared/http";
import { subscribe } from "../../../shared/stream";
import type { Call, IceServer, Media, Signal, SignalKind } from "../domain/model";

const conv = (id: string) => `/api/v1/conversations/${seg(id)}/call`;
const call = (id: string) => `/api/v1/calls/${seg(id)}`;

export const callGateway = {
  live: (conversationId: string) => request<{ call: Call | null }>("GET", conv(conversationId)).then((r) => r.call),
  join: (conversationId: string) => request<Call>("POST", conv(conversationId)),
  leave: (callId: string) => request<undefined>("POST", `${call(callId)}/leave`),
  media: (callId: string, m: Media) => request<Call>("PUT", `${call(callId)}/media`, m),
  signal: (callId: string, to: string, kind: SignalKind, data: string) =>
    request<undefined>("POST", `${call(callId)}/signal`, { to, kind, data }),
  ice: () => request<{ ice_servers: IceServer[] }>("GET", "/api/v1/calls/ice").then((r) => r.ice_servers),
};

/** call.updated and call.signal from the shared realtime stream. */
export function onCallEvents(onCall: (c: Call) => void, onSignal: (s: Signal) => void): () => void {
  return subscribe(["call.updated", "call.signal"], (type, data) => {
    if (type === "call.updated") onCall(data as Call);
    else onSignal(data as Signal);
  });
}
