import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { type FocusSession, api } from "../api";
import { EyeBreak } from "../eye/EyeBreak";

const FOCUS = ["focus"];
const mmss = (s: number) => `${String(Math.floor(Math.max(0, s) / 60)).padStart(2, "0")}:${String(Math.max(0, s) % 60).padStart(2, "0")}`;
const minutes = (s: number) => `${Math.round(s / 60)} min`;

// Pomodoro mode is a personal preference (on by default), shared by every component on the page.
const MODE_KEY = "kyber.pomodoro";
const MODE_EVENT = "kyber:pomodoro-mode";
function readMode(): boolean {
  try {
    return localStorage.getItem(MODE_KEY) !== "off";
  } catch {
    return true;
  }
}
function subscribeMode(cb: () => void) {
  window.addEventListener(MODE_EVENT, cb);
  window.addEventListener("storage", cb);
  return () => {
    window.removeEventListener(MODE_EVENT, cb);
    window.removeEventListener("storage", cb);
  };
}
export function usePomodoroMode(): [boolean, (on: boolean) => void] {
  const on = useSyncExternalStore(subscribeMode, readMode, () => true);
  const set = (next: boolean) => {
    try {
      localStorage.setItem(MODE_KEY, next ? "on" : "off");
    } catch {
      /* private mode: the toggle still works for this page */
    }
    window.dispatchEvent(new Event(MODE_EVENT));
  };
  return [on, set];
}

// Finishing a task (moving it to a Done status) is announced so eye health can react.
const DONE_EVENT = "kyber:issue-done";
export function announceIssueDone(issueKey: string) {
  window.dispatchEvent(new CustomEvent(DONE_EVENT, { detail: issueKey }));
}

/**
 * Milliseconds left, like a stopwatch: from the server's absolute deadline corrected for the
 * client's clock skew (server_time vs. the moment the response arrived); falls back to
 * remaining_seconds for older servers.
 */
function msLeft(s: FocusSession, receivedAt: number, now: number): number {
  if (s.state === "running" && s.ends_at && s.server_time) {
    const skew = Date.parse(s.server_time) - receivedAt; // server clock minus client clock
    return Date.parse(s.ends_at) - (now + skew);
  }
  const base = s.remaining_ms ?? s.remaining_seconds * 1000;
  return s.state === "running" ? base - (now - receivedAt) : base;
}

function useFocusActions() {
  const qc = useQueryClient();
  const done = (s?: FocusSession) =>
    Promise.all([qc.invalidateQueries({ queryKey: FOCUS }), s && qc.invalidateQueries({ queryKey: ["issue-focus", s.issue_key] })]);
  return { qc, done };
}

/** The running Pomodoro in the top bar: countdown plus pause / resume / stop (KYB-S41). */
export function FocusTimer() {
  const { done } = useFocusActions();
  const q = useQuery({ queryKey: FOCUS, queryFn: api.focus, refetchInterval: 30_000 });
  const [pomodoro, setPomodoro] = usePomodoroMode();
  const [, tick] = useState(0);
  const s = q.data;
  const running = s?.state === "running";
  const ms = s ? msLeft(s, q.dataUpdatedAt, Date.now()) : 0;
  // Re-render exactly when the displayed second changes (no setInterval drift).
  useEffect(() => {
    if (!running) return;
    const t = setTimeout(() => tick((n) => n + 1), Math.max(1, ((ms % 1000) + 1000) % 1000 || 1000));
    return () => clearTimeout(t);
  });
  // A hidden tab throttles timers: recompute as soon as it is visible again.
  useEffect(() => {
    const wake = () => tick((n) => n + 1);
    document.addEventListener("visibilitychange", wake);
    return () => document.removeEventListener("visibilitychange", wake);
  }, []);
  const act = useMutation({ mutationFn: api.focusAction, onSuccess: done });
  const [eyes, setEyes] = useState(false);
  const [ask, setAsk] = useState<string | null>(null);
  useEffect(() => {
    // Pomodoro off: offer the exercise when a task is finished. Pomodoro on: it comes at the break.
    const onDone = (e: Event) => {
      if (!readMode()) setAsk((e as CustomEvent<string>).detail);
    };
    window.addEventListener(DONE_EVENT, onDone);
    return () => window.removeEventListener(DONE_EVENT, onDone);
  }, []);
  const finished = useRef<string | null>(null);
  const left = Math.ceil(ms / 1000);
  useEffect(() => {
    if (s && running && left <= 0 && finished.current !== s.id) {
      finished.current = s.id; // once per session
      setEyes(true); // Pomodoro done: time to rest the eyes (Sharingan break)
      void done(s); // the server settles the session on the next read
    }
  });
  const eyeButton = (
    <>
      <button
        className={`ghost pomodoro-toggle${pomodoro ? " on" : ""}`}
        aria-label="Pomodoro mode"
        aria-pressed={pomodoro}
        title={pomodoro ? "Pomodoro mode is on" : "Pomodoro mode is off"}
        onClick={() => setPomodoro(!pomodoro)}
      >
        🍅
      </button>
      <button className="ghost eye-button" aria-label="Eye break" title="Eye break" onClick={() => setEyes(true)}>
        👁
      </button>
    </>
  );
  const breakScreen = (
    <>
      {ask && (
        <div role="alertdialog" aria-label="Eye exercise" aria-describedby="eye-ask" className="panel eye-ask">
          <p id="eye-ask">
            <b>{ask}</b> is done. Rest your eyes with a one-minute exercise?
          </p>
          <div className="row">
            <button
              onClick={() => {
                setAsk(null);
                setEyes(true);
              }}
            >
              Start exercise
            </button>
            <button className="ghost" onClick={() => setAsk(null)}>
              Not now
            </button>
          </div>
        </div>
      )}
      {eyes && <EyeBreak onClose={() => setEyes(false)} />}
    </>
  );
  if (!s)
    return (
      <>
        {eyeButton}
        {breakScreen}
      </>
    );
  return (
    <div className={`focus-timer ${s.state}`}>
      <span role="timer" aria-label={`Focus on ${s.issue_key}`}>
        <span aria-hidden="true">🍅</span> {s.issue_key} <b>{mmss(left)}</b>
      </span>
      {running ? (
        <button className="ghost" onClick={() => act.mutate("pause")}>
          Pause
        </button>
      ) : (
        <button className="ghost" onClick={() => act.mutate("resume")}>
          Resume
        </button>
      )}
      <button className="ghost" onClick={() => act.mutate("stop")}>
        Stop
      </button>
      {act.isError && <span role="alert" className="error">{act.error.message}</span>}
      {eyeButton}
      {breakScreen}
    </div>
  );
}

/** Start a Pomodoro on an issue and see its focus history (work log). */
export function IssueFocus({ issueKey }: { issueKey: string }) {
  const { done } = useFocusActions();
  const log = useQuery({ queryKey: ["issue-focus", issueKey], queryFn: () => api.issueFocus(issueKey) });
  const current = useQuery({ queryKey: FOCUS, queryFn: api.focus });
  const start = useMutation({ mutationFn: (m: number) => api.startFocus(issueKey, m), onSuccess: done });
  const [pomodoro] = usePomodoroMode();
  const busy = Boolean(current.data);
  return (
    <section className="issue-focus" aria-label="Focus">
      <h3>Focus</h3>
      {!pomodoro && <p className="muted">Pomodoro mode is off. Turn it on with 🍅 in the top bar.</p>}
      <div className="row" hidden={!pomodoro}>
        {[25, 15, 50].map((m) => (
          <button key={m} type="button" className={m === 25 ? "" : "ghost"} disabled={start.isPending || busy} onClick={() => start.mutate(m)}>
            Focus {m} min
          </button>
        ))}
      </div>
      {busy && <p className="muted">A focus session is running ({current.data?.issue_key}); stop it in the top bar to start another.</p>}
      {start.isError && <p role="alert" className="error">{start.error.message}</p>}
      {log.data && log.data.items.length > 0 && (
        <>
          <p className="muted">Total focus: {minutes(log.data.total_focused_seconds)}</p>
          <ul aria-label="Focus history" className="focus-log">
            {log.data.items.map((s) => (
              <li key={s.id}>
                <time dateTime={s.started_at}>{new Date(s.started_at).toLocaleString()}</time> · {minutes(s.focused_seconds)} ·{" "}
                <span className={`state ${s.state}`}>{s.state}</span>
                {s.reason && <span className="muted"> ({s.reason})</span>}
              </li>
            ))}
          </ul>
        </>
      )}
    </section>
  );
}
