// Live updates (ADR-0013): the server publishes "something changed in project X" hints over MQTT
// (over WebSocket in browsers); the app refetches its active queries through the normal,
// authorized API. Hints carry no content.
import { useQueryClient, type QueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { liveClient } from "./stream";

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

export { liveURL } from "./stream";

/** S88: the hints ride on the tab's one shared MQTT connection (shared/stream). */
const browserClient = (): Promise<LiveClient> => liveClient();

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
