// The realtime connection (ADR-0013, S88): one MQTT connection per tab carries both the caller's own events
// (chat, calls, whiteboards on kyber/<workspace>/users/<me>/events as {type, data}) and the board hints of
// live.ts (kyber/<workspace>/projects/<KEY>/events). The client reconnects and resubscribes by itself.
type Handler = (type: string, data: unknown) => void;
type TopicHandler = (topic: string, payload: Uint8Array) => void;

/** Every user topic of the caller; the broker delivers only the caller's own. */
export const USER_TOPIC = "kyber/+/users/+/events";

/** The part of an MQTT client the stream uses (mqtt.js in browsers; a fake in tests). */
export type StreamClient = {
  subscribe(topic: string | string[], done?: (err: Error | null) => void): unknown;
  on(event: "message", listener: (topic: string, payload: Uint8Array) => void): unknown;
  on(event: "connect", listener: () => void): unknown;
  end(force?: boolean): unknown;
};

/** liveURL is the broker's WebSocket endpoint on this origin (the session cookie goes along). */
export function liveURL(loc: Pick<Location, "protocol" | "host">): string {
  return `${loc.protocol === "https:" ? "wss" : "ws"}://${loc.host}/api/v1/mqtt`;
}

/** Connects mqtt.js lazily, so the library loads only when something subscribes. */
async function browserClient(): Promise<StreamClient> {
  const { default: mqtt } = await import("mqtt");
  return mqtt.connect(liveURL(window.location), {
    clientId: `web-${crypto.randomUUID()}`,
    reconnectPeriod: 2000,
    resubscribe: false, // we subscribe on every connect ourselves, to know when the subscriptions are active
  });
}

let connector: (() => Promise<StreamClient>) | undefined;

/** setStreamConnector replaces the MQTT connection (tests); undefined restores the browser's. */
export function setStreamConnector(c: (() => Promise<StreamClient>) | undefined): void {
  connector = c;
}

let client: Promise<StreamClient | undefined> | undefined;
let ready: StreamClient | undefined; // the connected client of `client`, once it resolved
const filters = new Map<string, Set<TopicHandler>>();
const connected = new Set<() => void>();

/**
 * Calls fn every time the stream connects or reconnects. Data loaded before that moment may have missed
 * events (MQTT does not replay), so listeners reload it.
 */
export function onConnect(fn: () => void): () => void {
  connected.add(fn);
  return () => void connected.delete(fn);
}

/** matches reports whether an MQTT topic filter (with single-level "+" wildcards) covers topic. */
function matches(filter: string, topic: string): boolean {
  const f = filter.split("/");
  const t = topic.split("/");
  return f.length === t.length && f.every((p, i) => p === "+" || p === t[i]);
}

function dispatch(topic: string, payload: Uint8Array) {
  for (const [filter, handlers] of [...filters]) {
    if (matches(filter, topic)) for (const fn of [...handlers]) fn(topic, payload);
  }
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
      // On every (re)connect: subscribe again and only then tell listeners to reload. Reloading earlier would leave
      // a gap: an event published before the broker granted the subscription would be neither loaded nor delivered.
      c.on("connect", () => {
        c.subscribe([...filters.keys()], (err) => {
          if (!err) for (const fn of [...connected]) fn();
        });
      });
      for (const filter of filters.keys()) c.subscribe(filter);
      ready = c;
      return c;
    },
    () => undefined, // realtime is best effort; pages still load and refetch normally
  );
  return pending;
}

/** Calls fn for every message on topics matching filter until the returned function is called. */
export function subscribeTopic(filter: string, fn: TopicHandler): () => void {
  let handlers = filters.get(filter);
  const fresh = !handlers;
  if (!handlers) filters.set(filter, (handlers = new Set()));
  handlers.add(fn);
  if (!client) client = open();
  else if (fresh) ready?.subscribe(filter); // still connecting: open() subscribes every filter

  return () => {
    handlers.delete(fn);
    if (handlers.size === 0) filters.delete(filter);
    if (filters.size === 0 && client) {
      const closing = client;
      client = undefined;
      ready = undefined;
      void closing.then((c) => c?.end(true));
    }
  };
}

/** Calls fn for every personal event of the given types until the returned function is called. */
export function subscribe(types: string[], fn: Handler): () => void {
  const wanted = new Set(types);
  return subscribeTopic(USER_TOPIC, (_topic, payload) => {
    let event: { type?: unknown; data?: unknown };
    try {
      event = JSON.parse(new TextDecoder().decode(payload)) as { type?: unknown; data?: unknown };
    } catch {
      return;
    }
    if (typeof event.type === "string" && wanted.has(event.type)) fn(event.type, event.data);
  });
}

/** liveClient gives live.ts an MQTT-like client over the shared connection: ending it only drops its topics. */
export function liveClient(): Promise<{ subscribe(topic: string): void; on(event: "message", l: () => void): void; end(): void }> {
  let listener: () => void = () => undefined;
  const stops: (() => void)[] = [];
  return Promise.resolve({
    subscribe: (topic: string) => void stops.push(subscribeTopic(topic, () => listener())),
    on: (_event: "message", l: () => void) => void (listener = l),
    end: () => stops.splice(0).forEach((stop) => stop()),
  });
}
