import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { api } from "../api";

const fmt = (iso: string) => new Date(iso).toLocaleString([], { weekday: "short", day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });

/** The workspace time table (KYB-S42). During a meeting its attendees cannot start a Pomodoro. */
export function MeetingsPage() {
  const qc = useQueryClient();
  const meetings = useQuery({ queryKey: ["meetings"], queryFn: () => api.meetings() });
  const me = useQuery({ queryKey: ["me"], queryFn: api.me });
  const projects = useQuery({ queryKey: ["projects"], queryFn: api.projects });
  const memberLists = useQueries({
    queries: (projects.data ?? []).map((p) => ({ queryKey: ["members", p.key], queryFn: () => api.members(p.key) })),
  });
  const people = new Map<string, string>();
  for (const q of memberLists) for (const m of q.data ?? []) people.set(m.user_id, m.name);
  if (me.data) people.delete(me.data.id); // the organizer always attends

  const [title, setTitle] = useState("");
  const [starts, setStarts] = useState("");
  const [mins, setMins] = useState(30);
  const [picked, setPicked] = useState<string[]>([]);
  const refresh = () => qc.invalidateQueries({ queryKey: ["meetings"] });
  const schedule = useMutation({
    mutationFn: () => {
      const start = new Date(starts);
      return api.scheduleMeeting({
        title, starts_at: start.toISOString(), ends_at: new Date(start.getTime() + mins * 60_000).toISOString(), attendee_ids: picked,
      });
    },
    onSuccess: () => {
      setTitle("");
      setPicked([]);
      return refresh();
    },
  });
  const cancel = useMutation({ mutationFn: api.cancelMeeting, onSuccess: refresh });

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    schedule.mutate();
  }

  return (
    <div className="meetings">
      <h1>Time table</h1>
      <p className="muted">Meetings for the next 7 days. While a meeting runs, its attendees cannot start a Pomodoro.</p>
      <ul aria-label="Meetings" className="panel meeting-list">
        {meetings.data?.length === 0 && <li className="muted">No meetings.</li>}
        {meetings.data?.map((m) => (
          <li key={m.id}>
            <b>{m.title}</b> · {fmt(m.starts_at)} – {new Date(m.ends_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })} ·{" "}
            <span className="muted">
              {m.attendee_ids.length} {m.attendee_ids.length === 1 ? "person" : "people"}
            </span>
            {m.organizer_id === me.data?.id && (
              <button className="ghost" aria-label={`Cancel ${m.title}`} onClick={() => cancel.mutate(m.id)}>
                Cancel
              </button>
            )}
          </li>
        ))}
      </ul>
      <form className="panel" onSubmit={onSubmit}>
        <h2>Schedule a meeting</h2>
        <label>
          Title
          <input value={title} onChange={(e) => setTitle(e.target.value)} required />
        </label>
        <label>
          Starts
          <input type="datetime-local" value={starts} onChange={(e) => setStarts(e.target.value)} required />
        </label>
        <label>
          Minutes
          <input type="number" min={5} max={480} value={mins} onChange={(e) => setMins(Number(e.target.value))} required />
        </label>
        <fieldset>
          <legend>Attendees</legend>
          {people.size === 0 && <span className="muted">Only you — add people to a project to invite them.</span>}
          {[...people].map(([id, name]) => (
            <label key={id} className="check">
              <input
                type="checkbox"
                checked={picked.includes(id)}
                onChange={(e) => setPicked((p) => (e.target.checked ? [...p, id] : p.filter((x) => x !== id)))}
              />
              {name}
            </label>
          ))}
        </fieldset>
        {schedule.isError && <p role="alert" className="error">{schedule.error.message}</p>}
        <button type="submit" disabled={schedule.isPending}>
          Schedule
        </button>
      </form>
    </div>
  );
}
