import { afterEach, describe, expect, it } from "vitest";
import { liveClient, onConnect, setStreamConnector, subscribe, subscribeTopic, USER_TOPIC } from "./stream";
import { FakeStreamClient, flush } from "./test/fakeStream";

afterEach(() => setStreamConnector(undefined));

describe("shared stream (S88: MQTT)", () => {
  it("multiplexes one MQTT connection across subscribers and ends it with the last one", async () => {
    const fake = new FakeStreamClient();
    setStreamConnector(fake.connector());
    const chat: string[] = [];
    const call: string[] = [];
    const stopChat = subscribe(["message.posted"], (type, data) => chat.push(`${type}:${JSON.stringify(data)}`));
    const stopCall = subscribe(["call.updated", "message.posted"], (type) => call.push(type));
    await flush();
    expect(fake.connects).toBe(1);
    expect(fake.topics).toEqual([USER_TOPIC]);
    fake.emit("message.posted", { id: "m-1" });
    fake.emit("call.updated", {});
    fake.message("kyber/w/projects/KYB/events", '{"project":"KYB"}'); // board hints are not stream events
    fake.message(USER_TOPIC, "not json");
    expect(chat).toEqual(['message.posted:{"id":"m-1"}']);
    expect(call).toEqual(["message.posted", "call.updated"]);
    stopChat();
    await flush();
    expect(fake.ended).toBe(false);
    stopCall();
    await flush();
    expect(fake.ended).toBe(true);
    const stopAgain = subscribe(["typing"], () => {});
    await flush();
    expect(fake.connects).toBe(2);
    stopAgain();
  });

  it("carries board hints and personal events on the same connection", async () => {
    const fake = new FakeStreamClient();
    setStreamConnector(fake.connector());
    const hints: string[] = [];
    const stopEvents = subscribe(["typing"], () => {});
    const stopHints = subscribeTopic("kyber/+/projects/+/events", (topic) => hints.push(topic));
    await flush();
    const late = subscribeTopic("kyber/+/projects/+/events", () => hints.push("late"));
    expect(fake.connects).toBe(1);
    expect(fake.topics).toEqual([USER_TOPIC, "kyber/+/projects/+/events"]); // one SUBSCRIBE per filter
    fake.message("kyber/w/projects/KYB/events", "{}");
    fake.emit("typing", {}); // a personal event is not a hint
    expect(hints).toEqual(["kyber/w/projects/KYB/events", "late"]);
    late();
    stopHints();
    stopEvents();
    await flush();
    expect(fake.ended).toBe(true);
  });

  it("offers live.ts a client over the shared connection", async () => {
    const fake = new FakeStreamClient();
    setStreamConnector(fake.connector());
    const c = await liveClient();
    let n = 0;
    c.on("message", () => n++);
    c.subscribe("kyber/+/projects/+/events");
    await flush();
    fake.message("kyber/w/projects/KYB/events", "{}");
    expect(n).toBe(1);
    c.end();
    await flush();
    expect(fake.ended).toBe(true);
  });

  it("tells listeners whenever the connection (re)connects, once its subscriptions are active", async () => {
    const fake = new FakeStreamClient();
    fake.holdAcks = true;
    setStreamConnector(fake.connector());
    let n = 0;
    const stopListen = onConnect(() => n++);
    const stop = subscribe(["typing"], () => {});
    await flush();
    fake.connect();
    expect(n).toBe(0); // events sent before the broker granted the subscription would be missed: reload only after
    fake.ack();
    expect(n).toBe(1);
    fake.connect(); // a reconnect subscribes again
    fake.ack();
    expect(n).toBe(2);
    stopListen();
    fake.connect();
    expect(n).toBe(2);
    stop();
  });

  it("ends a connection that arrives after everyone left, and survives a failed connect", async () => {
    const fake = new FakeStreamClient();
    setStreamConnector(fake.connector());
    subscribe(["typing"], () => {})();
    await flush();
    expect(fake.ended).toBe(true);
    setStreamConnector(() => Promise.reject(new Error("offline")));
    const stop = subscribe(["typing"], () => {});
    await flush();
    stop();
  });
});
