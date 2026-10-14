import { useMutation, useQueries, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { audienceLabel, calendarUseCases, RESPONSES, type MeetingResponse, tally } from "../application";
import { identityUseCases } from "../../identity/application";
import { projectUseCases } from "../../project/application";
import { useT } from "../../../shared/i18n/i18n";

const fmt = (iso: string) => new Date(iso).toLocaleString([], { weekday: "short", day: "numeric", month: "short", hour: "2-digit", minute: "2-digit" });

/** The workspace time table (KYB-S42). During a meeting its attendees cannot start a Pomodoro. */
export function MeetingsPage() {
  const t = useT();
  const qc = useQueryClient();
  const meetings = useQuery({ queryKey: ["meetings"], queryFn: () => calendarUseCases.meetings() });
  const me = useQuery({ queryKey: ["me"], queryFn: identityUseCases.me });
  const projects = useQuery({ queryKey: ["projects"], queryFn: projectUseCases.projects });
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
      <h1>{t("Time table")}</h1>
      <p className="muted">{t("Meetings for the next 7 days. While a meeting runs, its attendees cannot start a Pomodoro.")}</p>
      <ul aria-label={t("Meetings")} className="panel meeting-list">
        {meetings.data?.length === 0 && <li className="muted">{t("No meetings.")}</li>}
        {meetings.data?.map((m) => (
          <li key={m.id}>
            <b>{m.title}</b> · {fmt(m.starts_at)} – {new Date(m.ends_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })} ·{" "}
            {audienceLabel(m) && <span className="badge-label">{audienceLabel(m)}</span>}{" "}
            <span className="muted">
              {m.attendee_ids.length} {m.attendee_ids.length === 1 ? "person" : "people"}
              {tally(m) && ` · ${tally(m)}`}
            </span>
            {me.data && m.attendee_ids.includes(me.data.id) && m.organizer_id !== me.data.id && (
              <span className="rsvp" role="group" aria-label={t("Answer {title}", { title: m.title })}>
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
              <button className="ghost" aria-label={t("Cancel {title}", { title: m.title })} onClick={() => cancel.mutate(m.id)}>
                {t("Cancel")}
              </button>
            )}
          </li>
        ))}
      </ul>
      <form className="panel" onSubmit={onSubmit}>
        <h2>{t("Schedule a meeting")}</h2>
        <label>
          {t("Title")}
          <input value={title} onChange={(e) => setTitle(e.target.value)} required />
        </label>
        <label>
          {t("Starts")}
          <input type="datetime-local" value={starts} onChange={(e) => setStarts(e.target.value)} required />
        </label>
        <label>
          {t("Minutes")}
          <input type="number" min={5} max={480} value={mins} onChange={(e) => setMins(Number(e.target.value))} required />
        </label>
        <label>
          {t("Invite")}
          <select value={invite} onChange={(e) => setInvite(e.target.value)}>
            <option value="people">{t("Chosen people")}</option>
            {projects.data?.map((p) => (
              <option key={p.key} value={`project:${p.key}`}>
                {t("Everyone in {key}", { key: p.key })}
              </option>
            ))}
            <option value="workspace">{t("Everyone in the workspace")}</option>
          </select>
        </label>
        {invite !== "people" && <p className="muted">{t("Everyone invited is notified now and reminded 15 minutes before it starts.")}</p>}
        <fieldset hidden={invite !== "people"}>
          <legend>{t("Attendees")}</legend>
          {people.size === 0 && <span className="muted">{t("Only you — add people to a project to invite them.")}</span>}
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
          {t("Schedule")}
        </button>
      </form>
    </div>
  );
}
