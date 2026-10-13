import { QueryClient } from "@tanstack/react-query";
import { renderHook } from "@testing-library/react";
import { createElement, type ReactNode } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { type LiveSource, startLive, useLive } from "./live";

class FakeSource implements LiveSource {
  listeners = new Map<string, () => void>();
  closed = false;
  url = "";
  addEventListener(type: string, l: () => void) {
    this.listeners.set(type, l);
  }
  close() {
    this.closed = true;
  }
  emit(type: string) {
    this.listeners.get(type)?.();
  }
}

afterEach(() => vi.useRealTimers());

describe("live updates", () => {
  it("refetches active queries once per burst of hints and stops on close", () => {
    vi.useFakeTimers();
    const qc = new QueryClient();
    const invalidate = vi.spyOn(qc, "invalidateQueries").mockResolvedValue();
    const src = new FakeSource();
    const stop = startLive(qc, (url) => {
      src.url = url;
      return src;
    });
    expect(src.url).toBe("/api/v1/events");
    src.emit("change");
    src.emit("change");
    src.emit("resync");
    vi.advanceTimersByTime(300);
    expect(invalidate).toHaveBeenCalledTimes(1);
    expect(invalidate).toHaveBeenCalledWith({ refetchType: "active" });
    src.emit("change");
    stop();
    vi.advanceTimersByTime(300);
    expect(invalidate).toHaveBeenCalledTimes(1);
    expect(src.closed).toBe(true);
    startLive(qc, () => src)(); // closing with nothing pending
  });

  it("useLive opens one stream for the app and closes it on unmount", () => {
    const qc = new QueryClient();
    const src = new FakeSource();
    const open = vi.fn(() => src);
    const wrapper = ({ children }: { children: ReactNode }) => createElement(QueryClientProvider, { client: qc }, children);
    const { unmount } = renderHook(() => useLive(open), { wrapper });
    expect(open).toHaveBeenCalledTimes(1);
    unmount();
    expect(src.closed).toBe(true);
  });

  it("useLive does nothing where EventSource is missing (tests, old browsers)", () => {
    const qc = new QueryClient();
    const wrapper = ({ children }: { children: ReactNode }) => createElement(QueryClientProvider, { client: qc }, children);
    expect(() => renderHook(() => useLive(), { wrapper }).unmount()).not.toThrow();
  });
});
