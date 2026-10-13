// call: domain model — huddles (ADR-0009). Media flows browser to browser over a WebRTC mesh; the server
// keeps the participants and relays signaling. Pure types and rules, no I/O.
export type Media = { audio: boolean; video: boolean; screen: boolean };
export type Participant = Media & { user_id: string; joined_at: string };
export type PresentKind = "doc" | "whiteboard";
export type Presentation = { kind: PresentKind; ref: string; by: string };
export type Call = {
  id: string; conversation_id: string; started_by: string; started_at: string; live: boolean; participants: Participant[];
  presentation?: Presentation | null;
};
/** Something that can be presented to everyone in a huddle. */
export type Presentable = { kind: PresentKind; ref: string; title: string };
/** Where the presenter is looking: scroll position as fractions of the scrollable size, and zoom. */
export type FollowView = { call_id: string; x: number; y: number; zoom: number };

/** Turns a scroll position into the fractions others can apply to a differently sized viewport. */
export function toFraction(scroll: number, size: number, viewport: number): number {
  const range = size - viewport;
  return range <= 0 ? 0 : Math.min(1, Math.max(0, scroll / range));
}
export const fromFraction = (f: number, size: number, viewport: number) => Math.max(0, size - viewport) * f;
export type SignalKind = "offer" | "answer" | "candidate";
/** A relayed signal; data is ours (JSON of SignalData), opaque to the server. */
export type Signal = { call_id: string; from: string; kind: SignalKind; data: string };
export type SignalData = {
  description?: { type: "offer" | "answer" | "pranswer" | "rollback"; sdp?: string };
  candidate?: { candidate?: string; sdpMid?: string | null; sdpMLineIndex?: number | null; usernameFragment?: string | null };
  /** The id of the sender's screen-share stream, so the receiver can tell it from the camera. */
  screen?: string;
};
export type IceServer = { urls: string[]; username?: string; credential?: string };

export const MAX_PARTICIPANTS = 6;

export const me = (call: Call | null | undefined, user: string) => call?.participants.find((p) => p.user_id === user);

/** The other participants, to whom this browser keeps one peer connection each. */
export const peersOf = (call: Call | null | undefined, user: string) =>
  call?.live ? call.participants.filter((p) => p.user_id !== user).map((p) => p.user_id) : [];

/**
 * Perfect negotiation: when both sides offer at once, the polite peer rolls back and accepts the other's
 * offer. Exactly one of any two peers is polite.
 */
export const isPolite = (self: string, peer: string) => self > peer;

/**
 * Who opens the connection between two participants: the one who joined first (ties by id). The newcomer
 * waits for offers, so two fresh connections never offer at once (a collision there can stall ICE).
 */
export function opens(call: Call, self: string, peer: string): boolean {
  const a = call.participants.find((p) => p.user_id === self);
  const b = call.participants.find((p) => p.user_id === peer);
  if (!a || !b) return false;
  return a.joined_at === b.joined_at ? self < peer : a.joined_at < b.joined_at;
}

/** Who shares the screen right now (at most one person). */
export const presenter = (call: Call | null | undefined) => call?.participants.find((p) => p.screen)?.user_id;

/** The browser's WebRTC and media-device APIs, as a port (injectable for tests). */
export type Platform = {
  peer(config: RTCConfiguration): RTCPeerConnection;
  userMedia(c: MediaStreamConstraints): Promise<MediaStream>;
  displayMedia(): Promise<MediaStream>;
  devices(): Promise<MediaDeviceInfo[]>;
};
