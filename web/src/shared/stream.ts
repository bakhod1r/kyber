// The realtime stream (GET /api/v1/stream, Server-Sent Events): one EventSource per tab, shared by every
// context that subscribes to some event types. The browser reconnects by itself with Last-Event-ID.
type Handler = (type: string, data: unknown) => void;
type Sub = { types: Set<string>; fn: Handler };

let source: EventSource | undefined;
const listening = new Set<string>();
const subs = new Set<Sub>();

function listen(type: string) {
  if (!source || listening.has(type)) return;
  listening.add(type);
  source.addEventListener(type, (ev) => {
    const data: unknown = JSON.parse((ev as MessageEvent<string>).data);
    for (const s of [...subs]) if (s.types.has(type)) s.fn(type, data);
  });
}

/** Calls fn for every event of the given types until the returned function is called. */
export function subscribe(types: string[], fn: Handler): () => void {
  if (typeof EventSource === "undefined") return () => {};
  if (!source) source = new EventSource("/api/v1/stream", { withCredentials: true });
  const sub: Sub = { types: new Set(types), fn };
  subs.add(sub);
  for (const t of types) listen(t);
  return () => {
    subs.delete(sub);
    if (subs.size === 0 && source) {
      source.close();
      source = undefined;
      listening.clear();
    }
  };
}
