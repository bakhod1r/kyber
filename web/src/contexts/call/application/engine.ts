// call: the huddle engine — one RTCPeerConnection per other participant (full mesh), perfect negotiation
// over the server's signal relay, microphone/camera in one local stream and the screen share in another.
import { type Call, type IceServer, isPolite, type Media, me, opens, peersOf, type Platform, type Signal, type SignalData, type SignalKind } from "../domain/model";

export type Gateway = {
  join(conversationId: string): Promise<Call>;
  leave(callId: string): Promise<unknown>;
  media(callId: string, m: Media): Promise<Call>;
  signal(callId: string, to: string, kind: SignalKind, data: string): Promise<unknown>;
  ice(): Promise<IceServer[]>;
};

export type Remote = { camera?: MediaStream; screen?: MediaStream };
export type HuddleState = {
  call: Call | null;
  joined: boolean;
  local?: MediaStream;
  screen?: MediaStream;
  remotes: Record<string, Remote>;
  media: Media;
  error?: string;
};

type Peer = {
  pc: RTCPeerConnection; makingOffer: boolean; ignoreOffer: boolean; screenId?: string; streams: Map<string, MediaStream>;
  /** Signals to and from a peer are sent and handled one at a time, in order (an offer before its candidates). */
  out: Promise<unknown>; in: Promise<unknown>;
  watchdog?: ReturnType<typeof setTimeout>;
};

const OFF: Media = { audio: false, video: false, screen: false };

export class HuddleEngine {
  private state: HuddleState = { call: null, joined: false, remotes: {}, media: OFF };
  private peers = new Map<string, Peer>();
  private ice: IceServer[] = [];
  private local!: MediaStream; // created on join (MediaStream exists only in browsers)
  private screen?: MediaStream;
  private listeners = new Set<(s: HuddleState) => void>();
  /** Signals that arrive while we are still joining (the stream can beat the join's HTTP response). */
  private joining = false;
  private early: Signal[] = [];

  constructor(
    private readonly user: string,
    private readonly gw: Gateway,
    private readonly platform: Platform,
  ) {}

  get snapshot() {
    return this.state;
  }

  onChange(fn: (s: HuddleState) => void) {
    this.listeners.add(fn);
    return () => void this.listeners.delete(fn);
  }

  private set(patch: Partial<HuddleState>) {
    this.state = { ...this.state, ...patch };
    for (const fn of this.listeners) fn(this.state);
  }

  private fail(e: unknown) {
    this.set({ error: e instanceof Error ? e.message : String(e) });
  }

  /** Starts or joins the conversation's huddle with the microphone on (if the browser allows it). */
  async join(conversationId: string) {
    this.joining = true;
    this.early = [];
    try {
      this.ice = await this.gw.ice().catch(() => []);
      this.local = new MediaStream();
      let audio = false;
      try {
        const mic = await this.platform.userMedia({ audio: true });
        for (const t of mic.getAudioTracks()) this.local.addTrack(t);
        audio = true;
      } catch {
        // no microphone or permission denied: join muted
      }
      const call = await this.gw.join(conversationId);
      this.set({ joined: true, local: this.local, media: { audio, video: false, screen: false } });
      if (!audio) await this.gw.media(call.id, { audio, video: false, screen: false });
      this.update(call);
      for (const sig of this.early.splice(0)) void this.onSignal(sig);
    } catch (e) {
      this.fail(e);
    } finally {
      this.joining = false;
      this.early = [];
    }
  }

  /** Applies the server's view of the call: connects to newcomers, drops those who left. */
  update(call: Call | null) {
    this.set({ call });
    if (!this.state.joined || !call) return;
    if (!call.live || !me(call, this.user)) return this.teardown();
    const others = new Set(peersOf(call, this.user));
    for (const id of others) if (!this.peers.has(id) && opens(call, this.user, id)) this.connect(id);
    for (const [id, p] of this.peers) {
      if (!others.has(id)) {
        clearTimeout(p.watchdog);
        p.pc.close();
        this.peers.delete(id);
        const { [id]: _gone, ...rest } = this.state.remotes;
        this.set({ remotes: rest });
      }
    }
    // Someone else started sharing: the server stopped our share.
    if (this.screen && !me(call, this.user)?.screen) this.stopShare(false);
  }

  private send(to: string, kind: SignalKind, data: SignalData) {
    const id = this.state.call?.id;
    const peer = this.peers.get(to);
    if (!id || !peer) return;
    const body = JSON.stringify(data);
    peer.out = peer.out.then(() => this.gw.signal(id, to, kind, body)).catch((e: unknown) => this.fail(e));
  }

  private connect(id: string): Peer {
    const pc = this.platform.peer({ iceServers: this.ice });
    const peer: Peer = { pc, makingOffer: false, ignoreOffer: false, streams: new Map(), out: Promise.resolve(), in: Promise.resolve() };
    this.peers.set(id, peer);
    // Our own offers run in the same queue as the peer's signals, so an incoming offer never meets a
    // half-made local one (the polite side's rollback then sees a complete local offer).
    pc.onnegotiationneeded = () => {
      peer.in = peer.in.then(async () => {
        try {
          if (pc.signalingState !== "stable") return; // an offer is under way; renegotiation follows it
          peer.makingOffer = true;
          await pc.setLocalDescription();
          if (pc.localDescription) this.send(id, "offer", { description: pc.localDescription.toJSON(), screen: this.screen?.id });
        } catch (e) {
          this.fail(e);
        } finally {
          peer.makingOffer = false;
        }
      });
    };
    // Safety net: if ICE never starts after a negotiation (lost candidates) or fails (network change),
    // restart it, which gathers and exchanges fresh candidates.
    pc.oniceconnectionstatechange = () => {
      if (pc.iceConnectionState === "failed" && !isPolite(this.user, id)) pc.restartIce();
    };
    // Only the impolite side restarts, so two restarts never collide.
    pc.onsignalingstatechange = () => {
      if (pc.signalingState !== "stable" || isPolite(this.user, id)) return;
      clearTimeout(peer.watchdog);
      peer.watchdog = setTimeout(() => {
        if (pc.signalingState === "stable" && pc.iceConnectionState === "new" && pc.connectionState !== "closed") pc.restartIce();
      }, 2000);
    };
    pc.onicecandidate = ({ candidate }) => {
      if (candidate) this.send(id, "candidate", { candidate: candidate.toJSON() });
    };
    pc.ontrack = ({ streams }) => {
      for (const s of streams) peer.streams.set(s.id, s);
      this.relabel(id);
    };
    for (const t of this.local.getTracks()) pc.addTrack(t, this.local);
    if (this.screen) for (const t of this.screen.getTracks()) pc.addTrack(t, this.screen);
    return peer;
  }

  /** Splits a peer's incoming streams into camera/microphone and screen share. */
  private relabel(id: string) {
    const peer = this.peers.get(id);
    if (!peer) return;
    const remote: Remote = {};
    for (const [sid, s] of peer.streams) {
      if (s.getTracks().length === 0) continue;
      if (sid === peer.screenId) remote.screen = s;
      else remote.camera = s;
    }
    this.set({ remotes: { ...this.state.remotes, [id]: remote } });
  }

  /** Handles a relayed signal (perfect negotiation), in arrival order per peer. */
  onSignal(s: Signal): Promise<unknown> {
    if (this.joining && !this.state.joined) {
      this.early.push(s);
      return Promise.resolve();
    }
    if (!this.state.joined || s.call_id !== this.state.call?.id) return Promise.resolve();
    const peer = this.peers.get(s.from) ?? this.connect(s.from);
    peer.in = peer.in.then(() => this.handle(peer, s));
    return peer.in;
  }

  private async handle(peer: Peer, s: Signal) {
    const { pc } = peer;
    const data = JSON.parse(s.data) as SignalData;
    try {
      if (data.description) {
        if (data.description.type === "offer") {
          peer.screenId = data.screen;
          const collision = peer.makingOffer || pc.signalingState !== "stable";
          peer.ignoreOffer = !isPolite(this.user, s.from) && collision;
          if (peer.ignoreOffer) return;
        }
        await pc.setRemoteDescription(data.description);
        if (data.description.type === "offer") {
          await pc.setLocalDescription();
          if (pc.localDescription) this.send(s.from, "answer", { description: pc.localDescription.toJSON() });
        }
        this.relabel(s.from);
      } else if (data.candidate) {
        try {
          await pc.addIceCandidate(data.candidate);
        } catch (e) {
          if (!peer.ignoreOffer) throw e;
        }
      }
    } catch (e) {
      this.fail(e);
    }
  }

  private async publish(m: Media) {
    this.set({ media: m });
    const id = this.state.call?.id;
    if (id) this.update(await this.gw.media(id, m));
  }

  async toggleMic() {
    const tracks = this.local.getAudioTracks();
    if (tracks.length === 0) return this.fail(new Error("No microphone"));
    const on = !this.state.media.audio;
    for (const t of tracks) t.enabled = on;
    await this.publish({ ...this.state.media, audio: on });
  }

  private addTrack(t: MediaStreamTrack, stream: MediaStream) {
    stream.addTrack(t);
    for (const { pc } of this.peers.values()) pc.addTrack(t, stream);
  }

  private removeTrack(t: MediaStreamTrack, stream: MediaStream) {
    t.stop();
    stream.removeTrack(t);
    for (const { pc } of this.peers.values()) {
      const sender = pc.getSenders().find((x) => x.track === t);
      if (sender) pc.removeTrack(sender);
    }
  }

  async toggleCamera(deviceId?: string) {
    try {
      for (const t of this.local.getVideoTracks()) this.removeTrack(t, this.local);
      if (this.state.media.video && !deviceId) return await this.publish({ ...this.state.media, video: false });
      const cam = await this.platform.userMedia({ video: deviceId ? { deviceId: { exact: deviceId } } : true });
      for (const t of cam.getVideoTracks()) this.addTrack(t, this.local);
      this.set({ local: this.local });
      await this.publish({ ...this.state.media, video: true });
    } catch (e) {
      this.fail(e);
    }
  }

  /** Switches the microphone without renegotiating (replaceTrack). */
  async useMicrophone(deviceId: string) {
    try {
      const mic = await this.platform.userMedia({ audio: { deviceId: { exact: deviceId } } });
      const next = mic.getAudioTracks()[0];
      if (!next) return;
      next.enabled = this.state.media.audio;
      for (const old of this.local.getAudioTracks()) {
        for (const { pc } of this.peers.values()) await pc.getSenders().find((x) => x.track === old)?.replaceTrack(next);
        old.stop();
        this.local.removeTrack(old);
      }
      this.local.addTrack(next);
    } catch (e) {
      this.fail(e);
    }
  }

  async shareScreen() {
    try {
      if (this.screen) return await this.stopShare(true);
      const screen = await this.platform.displayMedia();
      this.screen = screen;
      for (const t of screen.getTracks()) {
        t.onended = () => void this.stopShare(true); // "Stop sharing" in the browser's own bar
        for (const { pc } of this.peers.values()) pc.addTrack(t, screen);
      }
      this.set({ screen });
      await this.publish({ ...this.state.media, screen: true });
    } catch (e) {
      this.fail(e);
    }
  }

  private async stopShare(tellServer: boolean) {
    const screen = this.screen;
    if (!screen) return;
    this.screen = undefined;
    for (const t of screen.getTracks()) this.removeTrack(t, screen);
    this.set({ screen: undefined });
    if (tellServer) await this.publish({ ...this.state.media, screen: false });
    else this.set({ media: { ...this.state.media, screen: false } });
  }

  devices() {
    return this.platform.devices();
  }

  private teardown() {
    for (const { pc, watchdog } of this.peers.values()) {
      clearTimeout(watchdog);
      pc.close();
    }
    this.peers.clear();
    for (const t of [...(this.local?.getTracks() ?? []), ...(this.screen?.getTracks() ?? [])]) t.stop();
    this.screen = undefined;
    this.set({ joined: false, local: undefined, screen: undefined, remotes: {}, media: OFF });
  }

  async leave() {
    if (!this.state.joined) return;
    const id = this.state.call?.id;
    this.teardown();
    if (id) await this.gw.leave(id).catch(() => {});
  }
}
