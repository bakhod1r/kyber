import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useEffect, useState } from "react";
import { audienceLabel, calendarUseCases, localInput, type Meeting, minutesOf, RESPONSES, type MeetingResponse, tally, when } from "../application";
import { identityUseCases } from "../../identity/application";
import { notifyUseCases } from "../../notify/application";
import { projectUseCases } from "../../project/application";

/** The browser's IANA time zone: meetings are shown in it and messages use it too. */
const zone = () => Intl.DateTimeFormat().resolvedOptions().timeZone;

/** RescheduleForm changes a meeting's title, start and length (organizer only). */
function RescheduleForm({ m, onDone }: { m: Meeting; onDone: () => void }) {
  const qc = useQueryClient();
  const [title, setTitle] = useState(m.title);
  const [starts, setStarts] = useState(localInput(m.starts_at, zone()));
  const [mins, setMins] = useState(minutesOf(m));
  const save = useMutation({
    mutationFn: () => {
      const start = new Date(starts);
      return calendarUseCases.rescheduleMeeting(m.id, {
        title, starts_at: start.toISOString(), ends_at: new Date(start.getTime() + mins * 60_000).toISOString(),
      });
    },
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["meetings"] });
      onDone();
    },
  });
  return (
    <form
      aria-label={`Reschedule ${m.title}`}
      className="reschedule"
      onSubmit={(e) => {
        e.preventDefault();
        save.mutate();
      }}
    >
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
      {save.isError && <p role="alert" className="error">{save.error.message}</p>}
      <button type="submit" disabled={save.isPending}>
        Save
      </button>
      <button type="button" className="ghost" onClick={onDone}>
        Close
      </button>
    </form>
  );
}

/** The workspace time table (KYB-S42). During a meeting its attendees cannot start a Pomodoro. */
export function MeetingsPage() {
  const qc = useQueryClient();
  const meetings = useQuery({ queryKey: ["meetings"], queryFn: () => calendarUseCases.meetings() });
  const me = useQuery({ queryKey: ["me"], queryFn: identityUseCases.me });
  const projects = useQuery({ queryKey: ["projects"], queryFn: projectUseCases.projects });
  // Remember the browser's zone so meeting emails and Telegram messages use it.
  const settings = useQuery({ queryKey: ["notification-settings"], queryFn: notifyUseCases.notificationSettings });
  const saveZone = useMutation({ mutationFn: (time_zone: string) => notifyUseCases.setNotificationSettings({ time_zone }) });
  const savedZone = settings.data?.time_zone;
  const { mutate: rememberZone } = saveZone;
  useEffect(() => {
    if (savedZone !== undefined && savedZone !== zone()) rememberZone(zone());
  }, [savedZone, rememberZone]);
  const [editing, setEditing] = useState<string | null>(null);
  const memberLists = useQueries({
    queries: (projects.data ?? []).map((p) => ({ queryKey: ["members", p.key], queryFn: () => projectUseCases.members(p.key) })),
  });
  const people = new Map<string, string>();
  for (const q of memberLists) for (const m of q.data ?? []) people.set(m.user_id, m.name);
  if (me.data) people.delete(me.data.id); // the organizer always attends

  const [title, setTitle] = useState("");
  const [starts, setStarts] = useState("");
  const [mins, setMins] = useState(30);
  const [picked, setPicked] = useState<string[]>([]);
  /** "people", "workspace" or "project:KEY". */
  const [invite, setInvite] = useState("people");
  const refresh = () => qc.invalidateQueries({ queryKey: ["meetings"] });
  const schedule = useMutation({
    mutationFn: () => {
      const start = new Date(starts);
      const [scope, project] = invite.split(":") as ["people" | "project" | "workspace", string | undefined];
      return calendarUseCases.scheduleMeeting({
        title, starts_at: start.toISOString(), ends_at: new Date(start.getTime() + mins * 60_000).toISOString(),
        attendee_ids: scope === "people" ? picked : [], scope, ...(project ? { project_key: project } : {}),
      });
    },
    onSuccess: () => {
      setTitle("");
      setPicked([]);
      return refresh();
    },
  });
  const cancel = useMutation({ mutationFn: calendarUseCases.cancelMeeting, onSuccess: refresh });
  const respond = useMutation({
    mutationFn: ({ id, r }: { id: string; r: MeetingResponse }) => calendarUseCases.respond(id, r),
    onSuccess: refresh,
  });

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
            <b>{m.title}</b> · <span>{when(m, zone())}</span> ·{" "}
            {audienceLabel(m) && <span className="badge-label">{audienceLabel(m)}</span>}{" "}
            <span className="muted">
              {m.attendee_ids.length} {m.attendee_ids.length === 1 ? "person" : "people"}
              {tally(m) && ` · ${tally(m)}`}
            </span>
            {me.data && m.attendee_ids.includes(me.data.id) && m.organizer_id !== me.data.id && (
              <span className="rsvp" role="group" aria-label={`Answer ${m.title}`}>
                {RESPONSES.map((r) => (
                  <button
                    key={r.id}
                    type="button"
                    className="ghost"
                    aria-pressed={m.responses?.[me.data.id] === r.id}
                    onClick={() => respond.mutate({ id: m.id, r: r.id })}
                  >
                    {r.label}
                  </button>
                ))}
              </span>
            )}
            {m.organizer_id === me.data?.id && (
              <>
                <button className="ghost" aria-label={`Reschedule ${m.title}`} aria-expanded={editing === m.id}
                  onClick={() => setEditing(editing === m.id ? null : m.id)}>
                  Reschedule
                </button>
                <button className="ghost" aria-label={`Cancel ${m.title}`} onClick={() => cancel.mutate(m.id)}>
                  Cancel
                </button>
              </>
            )}
            {editing === m.id && <RescheduleForm m={m} onDone={() => setEditing(null)} />}
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
        <label>
          Invite
          <select value={invite} onChange={(e) => setInvite(e.target.value)}>
            <option value="people">Chosen people</option>
            {projects.data?.map((p) => (
              <option key={p.key} value={`project:${p.key}`}>
                Everyone in {p.key}
              </option>
            ))}
            <option value="workspace">Everyone in the workspace</option>
          </select>
        </label>
        {invite !== "people" && <p className="muted">Everyone invited is notified now and reminded 15 minutes before it starts.</p>}
        <fieldset hidden={invite !== "people"}>
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
