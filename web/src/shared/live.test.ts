import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { LIVE_TOPIC, type LiveClient, liveURL, startLive, useLive } from "./live";

class FakeClient implements LiveClient {
  listener: (() => void) | undefined;
  topics: string[] = [];
  ended = false;
  subscribe(topic: string) {
    this.topics.push(topic);
  }
  on(_event: "message", l: () => void) {
    this.listener = l;
  }
  end() {
    this.ended = true;
  }
  emit() {
    this.listener?.();
  }
}

const wrapper =
  (qc: QueryClient) =>
  ({ children }: { children: ReactNode }) =>
    createElement(QueryClientProvider, { client: qc }, children);

afterEach(() => vi.useRealTimers());

describe("live updates", () => {
  it("refetches active queries once per burst of hints and disconnects on stop", () => {
    vi.useFakeTimers();
    const qc = new QueryClient();
    const invalidate = vi.spyOn(qc, "invalidateQueries").mockResolvedValue();
    const client = new FakeClient();
    const stop = startLive(qc, client);
    expect(client.topics).toEqual([LIVE_TOPIC]);
    client.emit();
    client.emit();
    vi.advanceTimersByTime(300);
    expect(invalidate).toHaveBeenCalledTimes(1);
    expect(invalidate).toHaveBeenCalledWith({ refetchType: "active" });
    client.emit();
    stop();
    vi.advanceTimersByTime(300);
    expect(invalidate).toHaveBeenCalledTimes(1);
    expect(client.ended).toBe(true);
    startLive(qc, new FakeClient())(); // stopping with nothing pending
  });

  it("builds the WebSocket URL from the page origin", () => {
    expect(liveURL({ protocol: "https:", host: "acme.kyber.dev" })).toBe("wss://acme.kyber.dev/api/v1/mqtt");
    expect(liveURL({ protocol: "http:", host: "localhost:8080" })).toBe("ws://localhost:8080/api/v1/mqtt");
  });

  it("useLive connects once for the app and disconnects on unmount", async () => {
    const client = new FakeClient();
    const connect = vi.fn(() => Promise.resolve<LiveClient>(client));
    const { unmount } = renderHook(() => useLive(connect), { wrapper: wrapper(new QueryClient()) });
    await waitFor(() => expect(client.topics).toEqual([LIVE_TOPIC]));
    expect(connect).toHaveBeenCalledTimes(1);
    unmount();
    expect(client.ended).toBe(true);
  });

  it("useLive drops a connection that arrives after unmount and ignores failures", async () => {
    const late = new FakeClient();
    let resolve: (c: LiveClient) => void = () => undefined;
    const { unmount } = renderHook(() => useLive(() => new Promise<LiveClient>((r) => (resolve = r))), { wrapper: wrapper(new QueryClient()) });
    unmount();
    resolve(late);
    await waitFor(() => expect(late.ended).toBe(true));
    expect(late.topics).toEqual([]);
    const failing = renderHook(() => useLive(() => Promise.reject(new Error("offline"))), { wrapper: wrapper(new QueryClient()) });
    await Promise.resolve();
    expect(() => failing.unmount()).not.toThrow();
  });

  it("useLive does nothing where WebSocket is missing", () => {
    vi.stubGlobal("WebSocket", undefined);
    try {
      expect(() => renderHook(() => useLive(), { wrapper: wrapper(new QueryClient()) }).unmount()).not.toThrow();
    } finally {
      vi.unstubAllGlobals();
    }
  });
});
