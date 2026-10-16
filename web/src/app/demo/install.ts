// Wires the GitHub Pages demo: the API is answered in the browser and there is no realtime broker.
import { setStreamConnector, type StreamClient } from "../../shared/stream";
import { DemoServer, type DemoStorage } from "./demoServer";

const STORAGE_KEY = "kyber.demo.v1";

/** browserStorage keeps the demo in localStorage; a blocked or full storage just forgets on reload. */
const browserStorage: DemoStorage = {
  load: () => {
    try {
      const raw = localStorage.getItem(STORAGE_KEY);
      return raw ? (JSON.parse(raw) as unknown) : undefined;
    } catch {
      return undefined;
    }
  },
  save: (state) => {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
    } catch {
      /* storage unavailable */
    }
  },
};

/** resetDemo forgets the visitor's changes and starts again from the seed data. */
export function resetDemo(): void {
  try {
    localStorage.removeItem(STORAGE_KEY);
  } catch {
    /* storage unavailable */
  }
  window.location.hash = "#/";
  window.location.reload();
}

type Fetch = typeof fetch;

/** demoFetch answers /api requests from the demo server and passes anything else to next. */
export function demoFetch(server: DemoServer, next: Fetch): Fetch {
  return async (input, init) => {
    const url = new URL(input instanceof Request ? input.url : String(input), window.location.href);
    if (!url.pathname.startsWith("/api/")) return next(input, init);
    const body = typeof init?.body === "string" && init.body ? (JSON.parse(init.body) as unknown) : undefined;
    const res = server.handle(init?.method ?? "GET", url.pathname + url.search, body);
    const empty = res.status === 204 || res.body === undefined;
    return new Response(empty ? null : JSON.stringify(res.body), {
      status: res.status,
      headers: { "Content-Type": res.status >= 400 ? "application/problem+json" : "application/json" },
    });
  };
}

/** A broker that never delivers: live updates have nobody else to come from in the demo. */
const silentStream: StreamClient = { subscribe: () => undefined, on: () => undefined, end: () => undefined };

export function installDemo(): void {
  window.fetch = demoFetch(new DemoServer(() => new Date(), browserStorage), window.fetch.bind(window));
  setStreamConnector(() => Promise.resolve(silentStream));
}
