// The realtime stream (S88: MQTT, ADR-0013): the caller's own events (chat, calls, whiteboards) arrive on
// kyber/<workspace>/users/<me>/events as {type, data}. One connection per tab, shared by every context that
// subscribes to some event types; the client reconnects and resubscribes by itself.
import { liveURL } from "./live";

type Handler = (type: string, data: unknown) => void;
type Sub = { types: Set<string>; fn: Handler };

/** Every user topic of the caller; the broker delivers only the caller's own. */
export const USER_TOPIC = "kyber/+/users/+/events";

/** The part of an MQTT client the stream uses (mqtt.js in browsers; a fake in tests). */
export type StreamClient = {
  subscribe(topic: string): unknown;
  on(event: "message", listener: (topic: string, payload: Uint8Array) => void): unknown;
  on(event: "connect", listener: () => void): unknown;
  end(force?: boolean): unknown;
};

/** Connects mqtt.js lazily, so the library loads only when something subscribes. */
async function browserClient(): Promise<StreamClient> {
  const { default: mqtt } = await import("mqtt");
  return mqtt.connect(liveURL(window.location), {
    clientId: `web-${crypto.randomUUID()}`,
    reconnectPeriod: 2000,
    resubscribe: true,
  });
}

let connector: (() => Promise<StreamClient>) | undefined;

/** setStreamConnector replaces the MQTT connection (tests); undefined restores the browser's. */
export function setStreamConnector(c: (() => Promise<StreamClient>) | undefined): void {
  connector = c;
}

let client: Promise<StreamClient | undefined> | undefined;
const subs = new Set<Sub>();
const connected = new Set<() => void>();

/**
 * Calls fn every time the stream connects or reconnects. Data loaded before that moment may have missed
 * events (MQTT does not replay), so listeners reload it.
 */
export function onConnect(fn: () => void): () => void {
  connected.add(fn);
  return () => void connected.delete(fn);
}

function dispatch(topic: string, payload: Uint8Array) {
  if (!/^kyber\/[^/]+\/users\/[^/]+\/events$/.test(topic)) return; // board hints are handled by live.ts
  let event: { type?: unknown; data?: unknown };
  try {
    event = JSON.parse(new TextDecoder().decode(payload)) as { type?: unknown; data?: unknown };
  } catch {
    return;
  }
  if (typeof event.type !== "string") return;
  for (const s of [...subs]) if (s.types.has(event.type)) s.fn(event.type, event.data);
}

function open(): Promise<StreamClient | undefined> {
  const connect = connector ?? (typeof WebSocket === "undefined" ? undefined : browserClient);
  if (!connect) return Promise.resolve(undefined);
  const pending = connect().then(
    (c) => {
      if (client !== pending) {
        c.end(true); // everyone left while it was connecting
        return undefined;
      }
      c.on("message", dispatch);
      c.on("connect", () => {
        for (const fn of [...connected]) fn();
      });
      c.subscribe(USER_TOPIC);
      return c;
    },
    () => undefined, // realtime is best effort; pages still load and refetch normally
  );
  return pending;
}

/** Calls fn for every event of the given types until the returned function is called. */
export function subscribe(types: string[], fn: Handler): () => void {
  const sub: Sub = { types: new Set(types), fn };
  subs.add(sub);
  client ??= open();
  return () => {
    subs.delete(sub);
    if (subs.size === 0 && client) {
      const closing = client;
      client = undefined;
      void closing.then((c) => c?.end(true));
    }
  };
}
