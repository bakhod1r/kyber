import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { Call, Media, Platform, RoomEvents, Signal } from "../domain/model";
import { HuddleEngine } from "./engine";

class FakeTrack {
  enabled = true;
  stopped = false;
  onended: (() => void) | null = null;
  constructor(readonly kind: "audio" | "video") {}
  stop() {
    this.stopped = true;
  }
}
let streams = 0;
class FakeStream {
  id = `s-${++streams}`;
  tracks: FakeTrack[] = [];
  constructor(tracks: FakeTrack[] = []) {
    this.tracks = [...tracks];
  }
  getTracks() { return this.tracks; }
  getAudioTracks() { return this.tracks.filter((t) => t.kind === "audio"); }
  getVideoTracks() { return this.tracks.filter((t) => t.kind === "video"); }
  addTrack(t: FakeTrack) { this.tracks.push(t); }
  removeTrack(t: FakeTrack) { this.tracks = this.tracks.filter((x) => x !== t); }
}
/** Records what the engine does with a peer connection. */
class FakePeer {
  signalingState: RTCSignalingState = "stable";
  localDescription: { type: string; toJSON(): object } | null = null;
  senders: { track: FakeTrack }[] = [];
  log: string[] = [];
  onnegotiationneeded: (() => void) | null = null;
  oniceconnectionstatechange: unknown = null;
  onsignalingstatechange: unknown = null;
  iceConnectionState = "new";
  connectionState = "new";
  restartIce() { this.log.push("restart"); }
  onicecandidate: unknown = null;
  ontrack: unknown = null;
  addTrack(t: FakeTrack) { this.senders.push({ track: t }); this.log.push(`add ${t.kind}`); }
  removeTrack(s: { track: FakeTrack }) { this.senders = this.senders.filter((x) => x !== s); this.log.push(`remove ${s.track.kind}`); }
  getSenders() { return this.senders; }
  async setLocalDescription() {
    const type = this.signalingState === "have-remote-offer" ? "answer" : "offer";
    this.signalingState = type === "offer" ? "have-local-offer" : "stable";
    this.localDescription = { type, toJSON: () => ({ type, sdp: `${type}-sdp` }) };
    this.log.push(`local ${type}`);
  }
  async setRemoteDescription(d: { type: string }) {
    this.signalingState = d.type === "offer" ? "have-remote-offer" : "stable";
    this.log.push(`remote ${d.type}`);
  }
  async addIceCandidate() { this.log.push("candidate"); }
  close() { this.log.push("close"); }
}

const callOf = (...ids: string[]): Call => ({
  id: "k-1", conversation_id: "c-1", started_by: ids[0] ?? "", started_at: "", live: true,
  participants: ids.map((user_id, i) => ({ user_id, joined_at: `2026-10-13T09:0${i}:00Z`, audio: true, video: false, screen: false })),
});

/** A fake media-server room: records what the engine publishes; tests drive its events. */
class FakeRoom {
  log: string[] = [];
  events!: RoomEvents;
  ticket = "";
  async publish(t: FakeTrack, screen: boolean) { this.log.push(`publish ${t.kind}${screen ? " screen" : ""}`); }
  async unpublish(t: FakeTrack) { this.log.push(`unpublish ${t.kind}`); }
  close() { this.log.push("close"); }
}

function setup(user: string, current: () => Call) {
  const peers: FakePeer[] = [];
  const rooms: FakeRoom[] = [];
  const sent: { to: string; kind: string; data: string }[] = [];
  const gw = {
    join: vi.fn(async () => current()),
    leave: vi.fn(async () => undefined),
    media: vi.fn(async (_id: string, _m: Media) => current()),
    signal: vi.fn(async (_id: string, to: string, kind: string, data: string) => void sent.push({ to, kind, data })),
    ice: vi.fn(async () => []),
    room: vi.fn(async (id: string) => ({ url: "wss://sfu.test", token: `t-${id}`, expires_at: "" })),
  };
  const platform: Platform = {
    peer: () => { const p = new FakePeer(); peers.push(p); return p as unknown as RTCPeerConnection; },
    userMedia: async (c) => new FakeStream([new FakeTrack(c.video ? "video" : "audio")]) as unknown as MediaStream,
    displayMedia: async () => new FakeStream([new FakeTrack("video")]) as unknown as MediaStream,
    devices: async () => [],
    room: async (url, token, events) => {
      const r = new FakeRoom();
      r.ticket = `${url} ${token}`;
      r.events = events;
      rooms.push(r);
      return r as unknown as Awaited<ReturnType<Platform["room"]>>;
    },
  };
  return { engine: new HuddleEngine(user, gw, platform), peers, sent, gw, rooms };
}

beforeEach(() => vi.stubGlobal("MediaStream", FakeStream));
afterEach(() => vi.unstubAllGlobals());

const offer = (from: string): Signal => ({ call_id: "k-1", from, kind: "offer", data: JSON.stringify({ description: { type: "offer", sdp: "x" }, screen: "scr" }) });

describe("huddle engine", () => {
  it("joins with the microphone; earlier participants open the connection to newcomers", async () => {
    const newcomer = setup("b", () => callOf("a", "b"));
    await newcomer.engine.join("c-1");
    expect(newcomer.peers).toHaveLength(0); // b waits for a's offer
    await newcomer.engine.onSignal(offer("a"));
    expect(newcomer.peers).toHaveLength(1);

    const { engine, peers, gw } = setup("a", () => callOf("a", "b"));
    await engine.join("c-1");
    expect(gw.join).toHaveBeenCalledWith("c-1");
    expect(engine.snapshot.media.audio).toBe(true);
    expect(peers).toHaveLength(1);
    expect(peers[0]?.log).toEqual(["add audio"]);
    engine.update(callOf("a")); // b left
    expect(peers[0]?.log.at(-1)).toBe("close");
  });

  it("answers an offer, and on glare only the polite peer yields", async () => {
    // "b" > "a": b is polite towards a.
    const polite = setup("b", () => callOf("b", "a")); // b came first here, so b connects
    await polite.engine.join("c-1");
    const pb = polite.peers[0]!;
    pb.onnegotiationneeded?.(); // b offers…
    await vi.waitFor(() => expect(pb.log).toContain("local offer"));
    await polite.engine.onSignal(offer("a")); // …while a's offer arrives: b accepts a's offer
    expect(pb.log).toContain("remote offer");
    expect(polite.sent.at(-1)?.kind).toBe("answer");

    const impolite = setup("a", () => callOf("a", "b"));
    await impolite.engine.join("c-1");
    const pa = impolite.peers[0]!;
    pa.onnegotiationneeded?.();
    await vi.waitFor(() => expect(pa.log).toContain("local offer"));
    await impolite.engine.onSignal(offer("b")); // a ignores b's colliding offer
    expect(pa.log).not.toContain("remote offer");
  });

  it("keeps an offer that arrives before its own join completes (the stream can beat the HTTP response)", async () => {
    let release!: () => void;
    const { engine, peers, gw, sent } = setup("a", () => callOf("a", "b"));
    gw.join.mockImplementationOnce(() => new Promise<Call>((ok) => { release = () => ok(callOf("a", "b")); }));
    const joining = engine.join("c-1");
    await vi.waitFor(() => expect(gw.join).toHaveBeenCalled());
    void engine.onSignal(offer("b")); // b's offer reaches us first
    release();
    await joining;
    await vi.waitFor(() => expect(peers[0]?.log).toContain("remote offer"));
    await vi.waitFor(() => expect(sent.some((x) => x.kind === "answer")).toBe(true));
  });

  it("restarts ICE when it never starts after a negotiation", async () => {
    vi.useFakeTimers();
    try {
      const { engine, peers } = setup("a", () => callOf("a", "b")); // a is impolite towards b
      await engine.join("c-1");
      const pc = peers[0]!;
      (pc.onsignalingstatechange as () => void)();
      vi.advanceTimersByTime(2100);
      expect(pc.log).toContain("restart");
      pc.iceConnectionState = "failed";
      (pc.oniceconnectionstatechange as () => void)();
      expect(pc.log.filter((l) => l === "restart")).toHaveLength(2);
    } finally {
      vi.useRealTimers();
    }
  });

  it("sends signals to a peer in order", async () => {
    const { engine, peers, sent } = setup("a", () => callOf("a", "b"));
    await engine.join("c-1");
    peers[0]?.onnegotiationneeded?.();
    await vi.waitFor(() => expect(peers[0]?.log).toContain("local offer"));
    (peers[0]?.onicecandidate as (e: { candidate: { toJSON(): object } }) => void)({ candidate: { toJSON: () => ({ candidate: "c1" }) } });
    await vi.waitFor(() => expect(sent.map((s) => s.kind)).toEqual(["offer", "candidate"]));
  });

  it("shares the screen as its own stream and stops when the server hands the screen to someone else", async () => {
    let current = callOf("a", "b");
    const { engine, peers, gw } = setup("a", () => current);
    await engine.join("c-1");
    current = { ...current, participants: current.participants.map((p) => ({ ...p, screen: p.user_id === "a" })) };
    await engine.shareScreen();
    expect(gw.media).toHaveBeenLastCalledWith("k-1", { audio: true, video: false, screen: true });
    expect(peers[0]?.log).toContain("add video");
    expect(engine.snapshot.screen).toBeDefined();
    engine.update({ ...current, participants: current.participants.map((p) => ({ ...p, screen: p.user_id === "b" })) });
    await vi.waitFor(() => expect(engine.snapshot.screen).toBeUndefined());
    expect(peers[0]?.log.at(-1)).toBe("remove video");
  });

  it("leaves: closes peers, stops tracks and tells the server", async () => {
    const { engine, peers, gw } = setup("a", () => callOf("a", "b"));
    await engine.join("c-1");
    await engine.leave();
    expect(peers[0]?.log.at(-1)).toBe("close");
    expect(gw.leave).toHaveBeenCalledWith("k-1");
    expect(engine.snapshot.joined).toBe(false);
    await engine.leave(); // not in a call any more: nothing to do
    expect(gw.leave).toHaveBeenCalledTimes(1);
  });

  it("in a room call publishes once to the media server instead of connecting to everyone (ADR-0017)", async () => {
    const room = (...ids: string[]): Call => ({ ...callOf(...ids), room: true });
    const { engine, peers, rooms, gw } = setup("a", () => room("a", "b", "c"));
    await engine.join("c-1");
    expect(peers).toHaveLength(0);
    expect(gw.room).toHaveBeenCalledWith("k-1");
    const r = rooms[0]!;
    gw.media.mockImplementation(async (_id: string, m: Media) => {
      const c = room("a", "b", "c");
      return { ...c, participants: c.participants.map((p) => (p.user_id === "a" ? { ...p, ...m } : p)) };
    });
    expect(r.ticket).toBe("wss://sfu.test t-k-1");
    expect(r.log).toEqual(["publish audio"]);

    const cam = new FakeStream([new FakeTrack("video")]) as unknown as MediaStream;
    r.events.track("b", "camera", cam);
    const scr = new FakeStream([new FakeTrack("video")]) as unknown as MediaStream;
    r.events.track("b", "screen", scr);
    expect(engine.snapshot.remotes.b).toEqual({ camera: cam, screen: scr });
    r.events.track("b", "screen", undefined); // stopped sharing
    expect(engine.snapshot.remotes.b).toEqual({ camera: cam });

    await engine.toggleCamera();
    await engine.shareScreen();
    expect(r.log).toEqual(["publish audio", "publish video", "publish video screen"]);
    await engine.toggleCamera();
    expect(r.log.at(-1)).toBe("unpublish video");
    await engine.onSignal(offer("b")); // nothing to negotiate in a room
    expect(peers).toHaveLength(0);

    r.events.left("b");
    expect(engine.snapshot.remotes.b).toBeUndefined();
    engine.update(room("a", "c"));
    expect(peers).toHaveLength(0);
    await engine.leave();
    expect(r.log.at(-1)).toBe("close");
    expect(gw.leave).toHaveBeenCalledWith("k-1");
  });

  it("a room that cannot be reached is an error, and the call is left", async () => {
    const { engine, gw } = setup("a", () => ({ ...callOf("a"), room: true }));
    gw.room.mockRejectedValueOnce(new Error("media server down"));
    await engine.join("c-1");
    expect(engine.snapshot.error).toBe("media server down");
    expect(engine.snapshot.joined).toBe(false);
    expect(gw.leave).toHaveBeenCalledWith("k-1");
  });

  it("keeps call updates that arrive while connecting to the room", async () => {
    const room = (...ids: string[]): Call => ({ ...callOf(...ids), room: true });
    const { engine, gw } = setup("b", () => room("a", "b"));
    gw.room.mockImplementationOnce(async (id: string) => {
      const newer = room("a", "b");
      newer.participants[0]!.video = true; // a turned the camera on meanwhile
      engine.update(newer);
      return { url: "wss://sfu.test", token: `t-${id}`, expires_at: "" };
    });
    await engine.join("c-1");
    expect(engine.snapshot.call?.participants[0]?.video).toBe(true);
  });
});
