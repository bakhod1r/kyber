import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState, useSyncExternalStore } from "react";
import { focusUseCases, type FocusSession } from "../application";
import { useT } from "../../../shared/i18n/i18n";

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

// Finishing a task (moving it to a Done status) is announced (shared/events).
export { announceIssueDone } from "../../../shared/events";

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
  const t = useT();
  const { done } = useFocusActions();
  const q = useQuery({ queryKey: FOCUS, queryFn: focusUseCases.focus, refetchInterval: 30_000 });
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
  const act = useMutation({ mutationFn: focusUseCases.focusAction, onSuccess: done });
  const finished = useRef<string | null>(null);
  const left = Math.ceil(ms / 1000);
  useEffect(() => {
    if (s && running && left <= 0 && finished.current !== s.id) {
      finished.current = s.id; // once per session
      void done(s); // the server settles the session on the next read
    }
  });
  const pomodoroToggle = (
    <button
      className={`ghost pomodoro-toggle${pomodoro ? " on" : ""}`}
      aria-label={t("Pomodoro mode")}
      aria-pressed={pomodoro}
      title={pomodoro ? t("Pomodoro mode is on") : t("Pomodoro mode is off")}
      onClick={() => setPomodoro(!pomodoro)}
    >
      🍅
    </button>
  );
  if (!s) return pomodoroToggle;
  return (
    <div className={`focus-timer ${s.state}`}>
      <span role="timer" aria-label={t("Focus on {key}", { key: s.issue_key })}>
        <span aria-hidden="true">🍅</span> {s.issue_key} <b>{mmss(left)}</b>
      </span>
      {running ? (
        <button className="ghost" onClick={() => act.mutate("pause")}>
          {t("Pause")}
        </button>
      ) : (
        <button className="ghost" onClick={() => act.mutate("resume")}>
          {t("Resume")}
        </button>
      )}
      <button className="ghost" onClick={() => act.mutate("stop")}>
        {t("Stop")}
      </button>
      {act.isError && <span role="alert" className="error">{act.error.message}</span>}
      {pomodoroToggle}
    </div>
  );
}

/** Start a Pomodoro on an issue and see its focus history (work log). */
export function IssueFocus({ issueKey }: { issueKey: string }) {
  const t = useT();
  const { done } = useFocusActions();
  const log = useQuery({ queryKey: ["issue-focus", issueKey], queryFn: () => focusUseCases.issueFocus(issueKey) });
  const current = useQuery({ queryKey: FOCUS, queryFn: focusUseCases.focus });
  const start = useMutation({ mutationFn: (m: number) => focusUseCases.startFocus(issueKey, m), onSuccess: done });
  const [pomodoro] = usePomodoroMode();
  const busy = Boolean(current.data);
  return (
    <section className="issue-focus" aria-label={t("Focus")}>
      <h3>{t("Focus")}</h3>
      {!pomodoro && <p className="muted">{t("Pomodoro mode is off. Turn it on with 🍅 in the top bar.")}</p>}
      <div className="row" hidden={!pomodoro}>
        {[25, 15, 50].map((m) => (
          <button key={m} type="button" className={m === 25 ? "" : "ghost"} disabled={start.isPending || busy} onClick={() => start.mutate(m)}>
            {t("Focus {n} min", { n: m })}
          </button>
        ))}
      </div>
      {busy && <p className="muted">{t("A focus session is running ({key}); stop it in the top bar to start another.", { key: current.data?.issue_key ?? "" })}</p>}
      {start.isError && <p role="alert" className="error">{start.error.message}</p>}
      {log.data && log.data.items.length > 0 && (
        <>
          <p className="muted">{t("Total focus: {time}", { time: minutes(log.data.total_focused_seconds) })}</p>
          <ul aria-label={t("Focus history")} className="focus-log">
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
