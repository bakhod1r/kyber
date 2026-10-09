import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { type FocusSession, api } from "../api";

const FOCUS = ["focus"];
const mmss = (s: number) => `${String(Math.floor(Math.max(0, s) / 60)).padStart(2, "0")}:${String(Math.max(0, s) % 60).padStart(2, "0")}`;
const minutes = (s: number) => `${Math.round(s / 60)} min`;

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
  const [, tick] = useState(0);
  const s = q.data;
  const running = s?.state === "running";
  useEffect(() => {
    if (!running) return;
    const t = setInterval(() => tick((n) => n + 1), 1000);
    return () => clearInterval(t);
  }, [running]);
  const act = useMutation({ mutationFn: api.focusAction, onSuccess: done });
  if (!s) return null;
  const left = running ? s.remaining_seconds - Math.floor((Date.now() - q.dataUpdatedAt) / 1000) : s.remaining_seconds;
  if (running && left <= 0) void done(s); // finished: the server settles it on the next read
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
    </div>
  );
}

/** Start a Pomodoro on an issue and see its focus history (work log). */
export function IssueFocus({ issueKey }: { issueKey: string }) {
  const { done } = useFocusActions();
  const log = useQuery({ queryKey: ["issue-focus", issueKey], queryFn: () => api.issueFocus(issueKey) });
  const current = useQuery({ queryKey: FOCUS, queryFn: api.focus });
  const start = useMutation({ mutationFn: (m: number) => api.startFocus(issueKey, m), onSuccess: done });
  const busy = Boolean(current.data);
  return (
    <section className="issue-focus" aria-label="Focus">
      <h3>Focus</h3>
      <div className="row">
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
