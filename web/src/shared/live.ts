// Live updates: the server streams "something changed in project X" hints (SSE, ADR-0008); the
// app refetches its active queries through the normal, authorized API. Hints carry no content.
import { useQueryClient, type QueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { subscribe } from "./stream";

/** A Server-Sent Events source (EventSource in browsers; a fake in tests). */
export type LiveSource = {
  addEventListener(type: string, listener: () => void): void;
  close(): void;
};

/** startLive refetches active queries at most once per `debounceMs` after hints arrive; the
 * returned function stops listening. The browser reconnects on its own (the server sends `retry`). */
export function startLive(qc: QueryClient, open: (url: string) => LiveSource, debounceMs = 300): () => void {
  const source = open("/api/v1/events");
  let timer: ReturnType<typeof setTimeout> | undefined;
  const refetch = () => {
    if (timer !== undefined) return;
    timer = setTimeout(() => {
      timer = undefined;
      void qc.invalidateQueries({ refetchType: "active" });
    }, debounceMs);
  };
  source.addEventListener("change", refetch);
  source.addEventListener("resync", refetch);
  return () => {
    if (timer !== undefined) clearTimeout(timer);
    source.close();
  };
}

/** S83: the hints ride on the shared realtime stream (/api/v1/stream), so a tab keeps one connection. */
const browserSource = (_url: string): LiveSource => {
  const stops: (() => void)[] = [];
  return {
    addEventListener: (type, listener) => void stops.push(subscribe([type], () => listener())),
    close: () => stops.splice(0).forEach((stop) => stop()),
  };
};

/** useLive keeps the signed-in app in sync with changes made by other people and sessions. */
export function useLive(open: (url: string) => LiveSource = browserSource): void {
  const qc = useQueryClient();
  useEffect(() => (typeof EventSource === "undefined" && open === browserSource ? undefined : startLive(qc, open)), [qc, open]);
}
