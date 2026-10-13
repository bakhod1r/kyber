import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { subscribe } from "./stream";

class FakeEventSource {
  static opened: FakeEventSource[] = [];
  listeners = new Map<string, (e: MessageEvent<string>) => void>();
  closed = false;
  constructor(readonly url: string) {
    FakeEventSource.opened.push(this);
  }
  addEventListener(type: string, fn: (e: MessageEvent<string>) => void) {
    this.listeners.set(type, fn);
  }
  close() {
    this.closed = true;
  }
  emit(type: string, data: unknown) {
    this.listeners.get(type)?.(new MessageEvent(type, { data: JSON.stringify(data) }));
  }
}

beforeEach(() => {
  FakeEventSource.opened = [];
  vi.stubGlobal("EventSource", FakeEventSource);
});
afterEach(() => vi.unstubAllGlobals());

describe("shared stream", () => {
  it("multiplexes one EventSource across subscribers and closes it with the last one", () => {
    const chat: string[] = [];
    const call: string[] = [];
    const stopChat = subscribe(["message.posted"], (type, data) => chat.push(`${type}:${JSON.stringify(data)}`));
    const stopCall = subscribe(["call.updated", "message.posted"], (type) => call.push(type));
    expect(FakeEventSource.opened).toHaveLength(1);
    const es = FakeEventSource.opened[0]!;
    es.emit("message.posted", { id: "m-1" });
    es.emit("call.updated", {});
    expect(chat).toEqual(['message.posted:{"id":"m-1"}']);
    expect(call).toEqual(["message.posted", "call.updated"]);
    stopChat();
    expect(es.closed).toBe(false);
    stopCall();
    expect(es.closed).toBe(true);
    subscribe(["typing"], () => {});
    expect(FakeEventSource.opened).toHaveLength(2);
  });
});
