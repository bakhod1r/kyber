import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type DragEvent, type FormEvent, useState } from "react";
import { type Issue, type Sprint, api } from "../api";

const DRAG_TYPE = "application/x-kyber-issue";

type Section = { id: string | null; title: string; sprint?: Sprint };
type Target = { section: string | null; beforeKey?: string };

export function Backlog({ projectKey, onOpen }: { projectKey: string; onOpen?: (issueKey: string) => void }) {
  const qc = useQueryClient();
  const sprints = useQuery({ queryKey: ["sprints", projectKey], queryFn: () => api.sprints(projectKey) });
  const issues = useQuery({ queryKey: ["issues", projectKey, "all"], queryFn: () => api.issues(projectKey) });
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [over, setOver] = useState<string | null>(null);

  const refresh = () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: ["issues", projectKey] }),
      qc.invalidateQueries({ queryKey: ["sprints", projectKey] }),
    ]);

  // A drop may need two calls: change sprint (if the section differs), then rank before the target row.
  const move = useMutation({
    mutationFn: async ({ issue, to }: { issue: Issue; to: Target }) => {
      if ((issue.sprint_id ?? null) !== to.section) {
        await api.editIssue(issue.key, { version: issue.version, sprint_id: to.section });
      }
      if (to.beforeKey && to.beforeKey !== issue.key) {
        await api.rankIssue(issue.key, { before: to.beforeKey });
      }
    },
    onMutate: () => setError(null),
    onError: (err) => setError(err.message),
    onSettled: refresh,
  });

  const start = useMutation({
    mutationFn: (id: string) => api.startSprint(id),
    onMutate: () => setError(null),
    onError: (err) => setError(err.message),
    onSettled: refresh,
  });
  const complete = useMutation({
    mutationFn: (id: string) => api.completeSprint(id),
    onMutate: () => {
      setError(null);
      setNotice(null);
    },
    onSuccess: (c) =>
      setNotice(`${c.sprint.name} completed: ${c.completed} done, ${c.returned} returned to the backlog.`),
    onError: (err) => setError(err.message),
    onSettled: refresh,
  });

  function drop(e: DragEvent, to: Target) {
    e.preventDefault();
    e.stopPropagation(); // a row drop must not also count as a drop on its section
    setOver(null);
    const key = e.dataTransfer.getData(DRAG_TYPE);
    const issue = issues.data?.find((i) => i.key === key);
    if (issue) move.mutate({ issue, to });
  }

  if (sprints.isPending || issues.isPending) return <p className="muted">Loading backlog…</p>;
  if (issues.isError) return <p role="alert" className="error">{issues.error.message}</p>;

  const open = (sprints.data ?? []).filter((s) => s.state !== "closed");
  const hasActive = open.some((s) => s.state === "active");
  const sections: Section[] = [
    ...open.map((s) => ({ id: s.id, title: s.name, sprint: s })),
    { id: null, title: "Backlog" },
  ];

  return (
    <div className="backlog">
      {error && <p role="alert" className="error">{error}</p>}
      {notice && <p role="status" className="notice">{notice}</p>}
      {sections.map((sec) => {
        const rows = issues.data.filter((i) => (i.sprint_id ?? null) === sec.id);
        const zone = sec.id ?? "backlog";
        return (
          <section
            key={zone}
            aria-label={sec.title}
            className={`backlog-section${over === zone ? " over" : ""}`}
            onDragOver={(e) => {
              e.preventDefault();
              setOver(zone);
            }}
            onDragLeave={() => setOver(null)}
            onDrop={(e) => drop(e, { section: sec.id })}
          >
            <header className="backlog-head">
              <div>
                <h2>
                  {sec.title} {sec.sprint && <span className={`state state-${sec.sprint.state}`}>{sec.sprint.state}</span>}
                  <span className="count">{rows.length}</span>
                </h2>
                {sec.sprint?.goal && <p className="muted goal">{sec.sprint.goal}</p>}
              </div>
              {sec.sprint?.state === "planned" && (
                <button
                  type="button"
                  disabled={hasActive || start.isPending}
                  title={hasActive ? "Complete the active sprint first" : undefined}
                  onClick={() => start.mutate(sec.sprint!.id)}
                >
                  Start sprint
                </button>
              )}
              {sec.sprint?.state === "active" && (
                <button type="button" disabled={complete.isPending} onClick={() => complete.mutate(sec.sprint!.id)}>
                  Complete sprint
                </button>
              )}
            </header>
            <ul className="backlog-rows">
              {rows.length === 0 && <li className="empty muted">Drag issues here</li>}
              {rows.map((i) => (
                <li
                  key={i.id}
                  draggable
                  className="backlog-row"
                  onDragStart={(e) => {
                    e.dataTransfer.setData(DRAG_TYPE, i.key);
                    e.dataTransfer.effectAllowed = "move";
                  }}
                  onDragOver={(e) => e.preventDefault()}
                  onDrop={(e) => drop(e, { section: sec.id, beforeKey: i.key })}
                >
                  <span className={`type type-${i.type}`}>{i.type}</span>
                  <button type="button" className="row-open" onClick={() => onOpen?.(i.key)}>
                    {i.title}
                  </button>
                  <span className={`status status-${i.status}`}>{i.status.replace("_", " ")}</span>
                  <span className={`prio prio-${i.priority}`} aria-label={`Priority: ${i.priority}`} />
                  <small className="key">{i.key}</small>
                </li>
              ))}
            </ul>
          </section>
        );
      })}
      <CreateSprint projectKey={projectKey} />
    </div>
  );
}

function CreateSprint({ projectKey }: { projectKey: string }) {
  const qc = useQueryClient();
  const [name, setName] = useState("");
  const [goal, setGoal] = useState("");
  const create = useMutation({
    mutationFn: () => api.createSprint(projectKey, name, goal),
    onSuccess: () => {
      setName("");
      setGoal("");
      return qc.invalidateQueries({ queryKey: ["sprints", projectKey] });
    },
  });
  function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (name.trim()) create.mutate();
  }
  return (
    <form className="panel inline create-sprint" onSubmit={onSubmit}>
      <h2>New sprint</h2>
      <label>
        Sprint name
        <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Sprint 1" maxLength={100} required />
      </label>
      <label>
        Goal
        <input value={goal} onChange={(e) => setGoal(e.target.value)} placeholder="What should this sprint achieve?" />
      </label>
      <button type="submit" disabled={create.isPending}>
        Create sprint
      </button>
      {create.isError && <p role="alert" className="error">{create.error.message}</p>}
    </form>
  );
}
