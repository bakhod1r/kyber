import { useT } from "../../../shared/i18n/i18n";
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
  const t = useT();
  const box = useRef<HTMLDivElement>(null);
  const [follow, setFollow] = useState(true);
  const last = useRef(0);
  useEffect(() => {
    const el = box.current;
    if (!el || mine || !follow || !view) return;
    el.scrollTo({ left: fromFraction(view.x, el.scrollWidth, el.clientWidth), top: fromFraction(view.y, el.scrollHeight, el.clientHeight) });
  }, [view, follow, mine]);
  return (
    <figure className="huddle-presentation" aria-label={t("Presentation")}>
      <figcaption className="row">
        <span>{mine ? t("You are presenting") : t("{name} is presenting", { name: by })} <b>{title}</b></span>
        {mine ? (
          <button className="ghost" onClick={onStop}>{t("Stop presenting")}</button>
        ) : (
          <label><input type="checkbox" checked={follow} onChange={(e) => setFollow(e.target.checked)} /> {t("Follow presenter")}</label>
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
export function Huddle({ conversationId, me, names, platform, presentables = [], renderPresentation, renderChat, autoJoin }: {
  conversationId: string; me: string; names: Map<string, string>; platform?: Platform;
  /** Docs and whiteboards that can be presented; renderPresentation draws the presented one. */
  presentables?: Presentable[]; renderPresentation?: RenderPresentation;
  /** The huddle's chat (S75), composed by the app shell. */
  renderChat?: (call: Call) => ReactNode;
  /** Join (or start) the huddle right away, e.g. from a meeting's "Join huddle" (S76). */
  autoJoin?: boolean;
}) {
  const t = useT();
  const h = useHuddle(conversationId, me, platform);
  const { call, state } = h;
  const [devices, setDevices] = useState<MediaDeviceInfo[]>([]);
  const [chatOpen, setChatOpen] = useState(false);
  const joining = useRef(false);
  useEffect(() => {
    if (autoJoin && !joining.current && !h.state.joined) {
      joining.current = true;
      void h.join();
    }
  }, [autoJoin, h]);
  const remoteAudio: Record<string, MediaStream | undefined> = {};
  for (const [id, r] of Object.entries(state.remotes)) remoteAudio[id] = r.camera;
  const speaking = useSpeaking({ ...remoteAudio, [me]: state.joined && state.media.audio ? state.local : undefined });
  const name = (id: string) => (id === me ? t("You") : (names.get(id) ?? t("Someone")));

  if (!state.joined) {
    const people = call?.participants ?? [];
    const full = people.length >= MAX_PARTICIPANTS;
    return (
      <div className="huddle-bar">
        {people.length > 0 && <span className="muted">🎧 {t("Huddle:")} {people.map((p) => name(p.user_id)).join(", ")}</span>}
        <button onClick={() => void h.join()} disabled={full} title={full ? t("The huddle is full (6 people)") : undefined}>
          {people.length > 0 ? t("Join huddle ({count})", { count: people.length }) : t("Start huddle")}
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
  const shownTitle = shown ? (presentables.find((x) => x.kind === shown.kind && x.ref === shown.ref)?.title ?? (shown.kind === "doc" ? t("a doc") : t("a whiteboard"))) : "";
  return (
    <section className="huddle" aria-label={t("Huddle")}>
      {call?.recording && <p className="huddle-recording" role="status">● {t("Recording by {name}", { name: name(call.recording.by) })}</p>}
      {h.saving && <p className="muted" role="status">{t("Saving the recording…")}</p>}
      {shown && renderPresentation && (
        <PresentationView p={shown} mine={shown.by === me} title={shownTitle} by={name(shown.by)} view={h.view}
          onScroll={(x, y) => h.follow(x, y)} onStop={() => void h.stopPresenting()} render={renderPresentation} />
      )}
      {shared && sharer && (
        <figure className="huddle-screen">
          <StreamVideo key={shared.getVideoTracks()[0]?.id ?? "none"} stream={shared} muted label={t("{name}'s screen", { name: name(sharer) })} />
          <figcaption>{t("{name} is presenting", { name: name(sharer) })}</figcaption>
        </figure>
      )}
      <ul className="huddle-grid" aria-label={t("Participants")}>
        {call?.participants.map((p) => {
          const stream = p.user_id === me ? state.local : state.remotes[p.user_id]?.camera;
          return (
            <li key={p.user_id} className={`huddle-tile ${speaking.has(p.user_id) ? "speaking" : ""}`}>
              {h.reactions.filter((r) => r.from === p.user_id).slice(-1).map((r) => (
                <span key={r.at} className="huddle-reaction" aria-label={t("{name} reacted {emoji}", { name: name(p.user_id), emoji: r.emoji })}>{r.emoji}</span>
              ))}
              {p.video ? <StreamVideo key={stream?.getVideoTracks()[0]?.id ?? "none"} stream={stream} muted={p.user_id === me} label={t("{name}'s camera", { name: name(p.user_id) })} /> : <span className="avatar">{name(p.user_id).slice(0, 1)}</span>}
              {p.user_id !== me && <StreamAudio stream={stream} />}
              <span className="huddle-name">
                {name(p.user_id)} {!p.audio && <span aria-label={t("muted")}>🔇</span>} {p.screen && <span aria-label={t("sharing screen")}>🖥️</span>}
                {p.hand && <span aria-label={t("hand raised")}> ✋</span>}
              </span>
            </li>
          );
        })}
      </ul>
      <div className="huddle-controls" role="toolbar" aria-label={t("Huddle controls")}>
        <button aria-pressed={state.media.audio} onClick={() => void h.toggleMic()}>{state.media.audio ? t("Mute") : t("Unmute")}</button>
        <button aria-pressed={state.media.video} onClick={() => void h.toggleCamera()}>{state.media.video ? t("Stop camera") : t("Start camera")}</button>
        <button aria-pressed={state.media.screen} onClick={() => void h.shareScreen()}>{state.media.screen ? t("Stop sharing") : t("Share screen")}</button>
        {renderPresentation && presentables.length > 0 && (
          <select aria-label={t("Present to everyone")} value="" onChange={(e) => {
            const [kind, ref] = e.target.value.split(":") as ["doc" | "whiteboard", string];
            if (ref) void h.present(kind, ref);
          }}>
            <option value="">{t("Present…")}</option>
            {presentables.map((x) => <option key={`${x.kind}:${x.ref}`} value={`${x.kind}:${x.ref}`}>{x.kind === "doc" ? "📄" : "🖍"} {x.title}</option>)}
          </select>
        )}
        {!call?.recording && <button onClick={() => void h.startRecording().catch(() => {})}>{t("Record")}</button>}
        {call?.recording?.by === me && <button aria-pressed onClick={() => void h.stopRecording()}>{t("Stop recording")}</button>}
        <button aria-pressed={!!mine?.hand} onClick={() => void h.raiseHand(!mine?.hand)}>{mine?.hand ? t("Lower hand") : `✋ ${t("Raise hand")}`}</button>
        <span className="row" role="group" aria-label={t("Reactions")}>
          {REACTIONS.map((e) => <button key={e} className="ghost" aria-label={t("React {emoji}", { emoji: e })} onClick={() => void h.react(e)}>{e}</button>)}
        </span>
        {renderChat && call && <button className="ghost" aria-expanded={chatOpen} onClick={() => setChatOpen((v) => !v)}>💬 {t("Huddle chat")}</button>}
        <button className="ghost" onClick={() => void pickDevices()}>{t("Devices")}</button>
        <button className="danger" onClick={() => void h.leave()}>{t("Leave")}</button>
      </div>
      {devices.length > 0 && (
        <div className="huddle-devices">
          <label>
            {t("Microphone")}{" "}
            <select onChange={(e) => void h.useMicrophone(e.target.value)} defaultValue="">
              <option value="" disabled>{t("Choose…")}</option>
              {devices.filter((d) => d.kind === "audioinput").map((d) => <option key={d.deviceId} value={d.deviceId}>{d.label || t("Microphone")}</option>)}
            </select>
          </label>
          <label>
            {t("Camera")}{" "}
            <select onChange={(e) => void h.toggleCamera(e.target.value)} defaultValue="">
              <option value="" disabled>{t("Choose…")}</option>
              {devices.filter((d) => d.kind === "videoinput").map((d) => <option key={d.deviceId} value={d.deviceId}>{d.label || t("Camera")}</option>)}
            </select>
          </label>
        </div>
      )}
      {chatOpen && renderChat && call && renderChat(call)}
      {state.error && <p role="alert" className="error">{state.error}</p>}
    </section>
  );
}
