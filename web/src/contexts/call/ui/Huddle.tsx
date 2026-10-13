import { useEffect, useRef, useState } from "react";
import { MAX_PARTICIPANTS, type Platform, presenter, useHuddle, useSpeaking } from "../application";

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
 * camera and screen-share controls. Media flows browser to browser (WebRTC mesh, ADR-0008).
 */
export function Huddle({ conversationId, me, names, platform }: {
  conversationId: string; me: string; names: Map<string, string>; platform?: Platform;
}) {
  const h = useHuddle(conversationId, me, platform);
  const { call, state } = h;
  const [devices, setDevices] = useState<MediaDeviceInfo[]>([]);
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

  const sharer = presenter(call);
  const shared = sharer === me ? state.screen : sharer ? state.remotes[sharer]?.screen : undefined;
  const pickDevices = async () => setDevices(devices.length ? [] : await h.devices().catch(() => []));
  return (
    <section className="huddle" aria-label="Huddle">
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
              {p.video ? <StreamVideo key={stream?.getVideoTracks()[0]?.id ?? "none"} stream={stream} muted={p.user_id === me} label={`${name(p.user_id)}'s camera`} /> : <span className="avatar">{name(p.user_id).slice(0, 1)}</span>}
              {p.user_id !== me && <StreamAudio stream={stream} />}
              <span className="huddle-name">
                {name(p.user_id)} {!p.audio && <span aria-label="muted">🔇</span>} {p.screen && <span aria-label="sharing screen">🖥️</span>}
              </span>
            </li>
          );
        })}
      </ul>
      <div className="huddle-controls" role="toolbar" aria-label="Huddle controls">
        <button aria-pressed={state.media.audio} onClick={() => void h.toggleMic()}>{state.media.audio ? "Mute" : "Unmute"}</button>
        <button aria-pressed={state.media.video} onClick={() => void h.toggleCamera()}>{state.media.video ? "Stop camera" : "Start camera"}</button>
        <button aria-pressed={state.media.screen} onClick={() => void h.shareScreen()}>{state.media.screen ? "Stop sharing" : "Share screen"}</button>
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
      {state.error && <p role="alert" className="error">{state.error}</p>}
    </section>
  );
}
