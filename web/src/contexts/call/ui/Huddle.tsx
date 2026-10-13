import { type ReactNode, useEffect, useRef, useState } from "react";
import {
  type Call, fromFraction, MAX_PARTICIPANTS, type Platform, type Presentable, type Presentation, presenter, REACTIONS, toFraction, useHuddle, useSpeaking,
} from "../application";

/** The app shell renders presented files (docs, whiteboards), which belong to other contexts. */
export type RenderPresentation = (p: Presentation) => ReactNode;

/**
 * The file presented to everyone (S60). The presenter's scrolling is shared; others follow it unless they
 * turn following off to look around on their own.
 */
function PresentationView({ p, mine, title, by, view, onScroll, onStop, render }: {
  p: Presentation; mine: boolean; title: string; by: string; view: { x: number; y: number } | null;
  onScroll: (x: number, y: number) => void; onStop: () => void; render: RenderPresentation;
}) {
  const box = useRef<HTMLDivElement>(null);
  const [follow, setFollow] = useState(true);
  const last = useRef(0);
  useEffect(() => {
    const el = box.current;
    if (!el || mine || !follow || !view) return;
    el.scrollTo({ left: fromFraction(view.x, el.scrollWidth, el.clientWidth), top: fromFraction(view.y, el.scrollHeight, el.clientHeight) });
  }, [view, follow, mine]);
  return (
    <figure className="huddle-presentation" aria-label="Presentation">
      <figcaption className="row">
        <span>{mine ? "You are presenting" : `${by} is presenting`} <b>{title}</b></span>
        {mine ? (
          <button className="ghost" onClick={onStop}>Stop presenting</button>
        ) : (
          <label><input type="checkbox" checked={follow} onChange={(e) => setFollow(e.target.checked)} /> Follow presenter</label>
        )}
      </figcaption>
      <div ref={box} className="huddle-presentation-body" data-testid="presentation-scroll"
        onScroll={() => {
          const el = box.current;
          if (!mine || !el || Date.now() - last.current < 120) return;
          last.current = Date.now();
          onScroll(toFraction(el.scrollLeft, el.scrollWidth, el.clientWidth), toFraction(el.scrollTop, el.scrollHeight, el.clientHeight));
        }}>
        {render(p)}
      </div>
    </figure>
  );
}

/** A <video> bound to a MediaStream (our own preview is muted to avoid echo). */
function StreamVideo({ stream, muted, label }: { stream?: MediaStream; muted?: boolean; label: string }) {
  const ref = useRef<HTMLVideoElement>(null);
  useEffect(() => {
    if (ref.current && ref.current.srcObject !== (stream ?? null)) ref.current.srcObject = stream ?? null;
  }, [stream]);
  return <video ref={ref} autoPlay playsInline muted={muted} aria-label={label} />;
}

/** Remote audio when the camera is off still has to play. */
function StreamAudio({ stream }: { stream?: MediaStream }) {
  const ref = useRef<HTMLAudioElement>(null);
  useEffect(() => {
    if (ref.current && ref.current.srcObject !== (stream ?? null)) ref.current.srcObject = stream ?? null;
  }, [stream]);
  return <audio ref={ref} autoPlay />;
}

/**
 * Zoom-style huddle of a conversation (S57): start or join, then a grid of participants with microphone,
 * camera and screen-share controls. Media flows browser to browser (WebRTC mesh, ADR-0009).
 */
export function Huddle({ conversationId, me, names, platform, presentables = [], renderPresentation, renderChat }: {
  conversationId: string; me: string; names: Map<string, string>; platform?: Platform;
  /** Docs and whiteboards that can be presented; renderPresentation draws the presented one. */
  presentables?: Presentable[]; renderPresentation?: RenderPresentation;
  /** The huddle's chat (S75), composed by the app shell. */
  renderChat?: (call: Call) => ReactNode;
}) {
  const h = useHuddle(conversationId, me, platform);
  const { call, state } = h;
  const [devices, setDevices] = useState<MediaDeviceInfo[]>([]);
  const [chatOpen, setChatOpen] = useState(false);
  const remoteAudio: Record<string, MediaStream | undefined> = {};
  for (const [id, r] of Object.entries(state.remotes)) remoteAudio[id] = r.camera;
  const speaking = useSpeaking({ ...remoteAudio, [me]: state.joined && state.media.audio ? state.local : undefined });
  const name = (id: string) => (id === me ? "You" : (names.get(id) ?? "Someone"));

  if (!state.joined) {
    const people = call?.participants ?? [];
    const full = people.length >= MAX_PARTICIPANTS;
    return (
      <div className="huddle-bar">
        {people.length > 0 && <span className="muted">🎧 Huddle: {people.map((p) => name(p.user_id)).join(", ")}</span>}
        <button onClick={() => void h.join()} disabled={full} title={full ? "The huddle is full (6 people)" : undefined}>
          {people.length > 0 ? `Join huddle (${people.length})` : "Start huddle"}
        </button>
        {state.error && <span role="alert" className="error">{state.error}</span>}
      </div>
    );
  }

  const mine = call?.participants.find((p) => p.user_id === me);
  const sharer = presenter(call);
  const shared = sharer === me ? state.screen : sharer ? state.remotes[sharer]?.screen : undefined;
  const pickDevices = async () => setDevices(devices.length ? [] : await h.devices().catch(() => []));
  const shown = call?.presentation;
  const shownTitle = shown ? (presentables.find((x) => x.kind === shown.kind && x.ref === shown.ref)?.title ?? (shown.kind === "doc" ? "a doc" : "a whiteboard")) : "";
  return (
    <section className="huddle" aria-label="Huddle">
      {call?.recording && <p className="huddle-recording" role="status">● Recording by {name(call.recording.by)}</p>}
      {h.saving && <p className="muted" role="status">Saving the recording…</p>}
      {shown && renderPresentation && (
        <PresentationView p={shown} mine={shown.by === me} title={shownTitle} by={name(shown.by)} view={h.view}
          onScroll={(x, y) => h.follow(x, y)} onStop={() => void h.stopPresenting()} render={renderPresentation} />
      )}
      {shared && sharer && (
        <figure className="huddle-screen">
          <StreamVideo key={shared.getVideoTracks()[0]?.id ?? "none"} stream={shared} muted label={`${name(sharer)}'s screen`} />
          <figcaption>{name(sharer)} is presenting</figcaption>
        </figure>
      )}
      <ul className="huddle-grid" aria-label="Participants">
        {call?.participants.map((p) => {
          const stream = p.user_id === me ? state.local : state.remotes[p.user_id]?.camera;
          return (
            <li key={p.user_id} className={`huddle-tile ${speaking.has(p.user_id) ? "speaking" : ""}`}>
              {h.reactions.filter((r) => r.from === p.user_id).slice(-1).map((r) => (
                <span key={r.at} className="huddle-reaction" aria-label={`${name(p.user_id)} reacted ${r.emoji}`}>{r.emoji}</span>
              ))}
              {p.video ? <StreamVideo key={stream?.getVideoTracks()[0]?.id ?? "none"} stream={stream} muted={p.user_id === me} label={`${name(p.user_id)}'s camera`} /> : <span className="avatar">{name(p.user_id).slice(0, 1)}</span>}
              {p.user_id !== me && <StreamAudio stream={stream} />}
              <span className="huddle-name">
                {name(p.user_id)} {!p.audio && <span aria-label="muted">🔇</span>} {p.screen && <span aria-label="sharing screen">🖥️</span>}
                {p.hand && <span aria-label="hand raised"> ✋</span>}
              </span>
            </li>
          );
        })}
      </ul>
      <div className="huddle-controls" role="toolbar" aria-label="Huddle controls">
        <button aria-pressed={state.media.audio} onClick={() => void h.toggleMic()}>{state.media.audio ? "Mute" : "Unmute"}</button>
        <button aria-pressed={state.media.video} onClick={() => void h.toggleCamera()}>{state.media.video ? "Stop camera" : "Start camera"}</button>
        <button aria-pressed={state.media.screen} onClick={() => void h.shareScreen()}>{state.media.screen ? "Stop sharing" : "Share screen"}</button>
        {renderPresentation && presentables.length > 0 && (
          <select aria-label="Present to everyone" value="" onChange={(e) => {
            const [kind, ref] = e.target.value.split(":") as ["doc" | "whiteboard", string];
            if (ref) void h.present(kind, ref);
          }}>
            <option value="">Present…</option>
            {presentables.map((x) => <option key={`${x.kind}:${x.ref}`} value={`${x.kind}:${x.ref}`}>{x.kind === "doc" ? "📄" : "🖍"} {x.title}</option>)}
          </select>
        )}
        {!call?.recording && <button onClick={() => void h.startRecording().catch(() => {})}>Record</button>}
        {call?.recording?.by === me && <button aria-pressed onClick={() => void h.stopRecording()}>Stop recording</button>}
        <button aria-pressed={!!mine?.hand} onClick={() => void h.raiseHand(!mine?.hand)}>{mine?.hand ? "Lower hand" : "✋ Raise hand"}</button>
        <span className="row" role="group" aria-label="Reactions">
          {REACTIONS.map((e) => <button key={e} className="ghost" aria-label={`React ${e}`} onClick={() => void h.react(e)}>{e}</button>)}
        </span>
        {renderChat && call && <button className="ghost" aria-expanded={chatOpen} onClick={() => setChatOpen((v) => !v)}>💬 Huddle chat</button>}
        <button className="ghost" onClick={() => void pickDevices()}>Devices</button>
        <button className="danger" onClick={() => void h.leave()}>Leave</button>
      </div>
      {devices.length > 0 && (
        <div className="huddle-devices">
          <label>
            Microphone{" "}
            <select onChange={(e) => void h.useMicrophone(e.target.value)} defaultValue="">
              <option value="" disabled>Choose…</option>
              {devices.filter((d) => d.kind === "audioinput").map((d) => <option key={d.deviceId} value={d.deviceId}>{d.label || "Microphone"}</option>)}
            </select>
          </label>
          <label>
            Camera{" "}
            <select onChange={(e) => void h.toggleCamera(e.target.value)} defaultValue="">
              <option value="" disabled>Choose…</option>
              {devices.filter((d) => d.kind === "videoinput").map((d) => <option key={d.deviceId} value={d.deviceId}>{d.label || "Camera"}</option>)}
            </select>
          </label>
        </div>
      )}
      {chatOpen && renderChat && call && renderChat(call)}
      {state.error && <p role="alert" className="error">{state.error}</p>}
    </section>
  );
}
