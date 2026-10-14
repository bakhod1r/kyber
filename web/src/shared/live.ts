// Live updates (ADR-0013): the server publishes "something changed in project X" hints over MQTT
// (over WebSocket in browsers); the app refetches its active queries through the normal,
// authorized API. Hints carry no content.
import { useQueryClient, type QueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

/** Every project of the caller's workspace; the broker delivers only browsable projects. */
export const LIVE_TOPIC = "kyber/+/projects/+/events";

/** The part of an MQTT client the app uses (mqtt.js in browsers; a fake in tests). */
export type LiveClient = {
  subscribe(topic: string): unknown;
  on(event: "message", listener: () => void): unknown;
  end(force?: boolean): unknown;
};

/** startLive refetches active queries at most once per `debounceMs` after hints arrive; the
 * returned function disconnects. The client reconnects (and resubscribes) on its own. */
export function startLive(qc: QueryClient, client: LiveClient, debounceMs = 300): () => void {
  let timer: ReturnType<typeof setTimeout> | undefined;
  client.on("message", () => {
    if (timer !== undefined) return;
    timer = setTimeout(() => {
      timer = undefined;
      void qc.invalidateQueries({ refetchType: "active" });
    }, debounceMs);
  });
  client.subscribe(LIVE_TOPIC);
  return () => {
    if (timer !== undefined) clearTimeout(timer);
    client.end(true);
  };
}

/** liveURL is the broker's WebSocket endpoint on this origin (the session cookie goes along). */
export function liveURL(loc: Pick<Location, "protocol" | "host">): string {
  return `${loc.protocol === "https:" ? "wss" : "ws"}://${loc.host}/api/v1/mqtt`;
}

/** Connects mqtt.js lazily, so the library loads only for signed-in pages. */
async function browserClient(): Promise<LiveClient> {
  const { default: mqtt } = await import("mqtt");
  return mqtt.connect(liveURL(window.location), {
    clientId: `web-${crypto.randomUUID()}`,
    reconnectPeriod: 3000,
    resubscribe: true,
  });
}

/** useLive keeps the signed-in app in sync with changes made by other people and sessions. */
export function useLive(connect: () => Promise<LiveClient> = browserClient): void {
  const qc = useQueryClient();
  useEffect(() => {
    if (connect === browserClient && typeof WebSocket === "undefined") return undefined;
    let stop: (() => void) | undefined;
    let cancelled = false;
    connect().then(
      (client) => {
        if (cancelled) client.end(true);
        else stop = startLive(qc, client);
      },
      () => undefined, // live updates are best effort; pages still load and refetch normally
    );
    return () => {
      cancelled = true;
      stop?.();
    };
  }, [qc, connect]);
}
