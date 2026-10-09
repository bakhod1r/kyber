import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type DragEvent, type FormEvent, useState } from "react";
import { ISSUE_TYPES, type Issue, type IssueType, STATUSES, type Status, api } from "../api";

const DRAG_TYPE = "application/x-kyber-issue";

export function Board({ projectKey }: { projectKey: string }) {
  const qc = useQueryClient();
  const queryKey = ["issues", projectKey];
  const issues = useQuery({ queryKey, queryFn: () => api.issues(projectKey) });
  const [error, setError] = useState<string | null>(null);
  const [over, setOver] = useState<Status | null>(null);

  const move = useMutation({
    mutationFn: ({ key, to }: { key: string; to: Status }) => api.transition(key, to),
    // Optimistic: move the card now, roll back on rejection.
    onMutate: async ({ key, to }) => {
      setError(null);
      await qc.cancelQueries({ queryKey });
      const previous = qc.getQueryData<Issue[]>(queryKey);
      qc.setQueryData<Issue[]>(queryKey, (list) => list?.map((i) => (i.key === key ? { ...i, status: to } : i)));
      return { previous };
    },
    onError: (err, _vars, ctx) => {
      qc.setQueryData(queryKey, ctx?.previous);
      setError(err.message);
    },
    onSettled: () => qc.invalidateQueries({ queryKey }),
  });

  function onDrop(e: DragEvent, to: Status) {
    e.preventDefault();
    setOver(null);
    const key = e.dataTransfer.getData(DRAG_TYPE);
    const issue = issues.data?.find((i) => i.key === key);
    if (issue && issue.status !== to) move.mutate({ key, to });
  }

  if (issues.isPending) return <p className="muted">Loading board…</p>;
  if (issues.isError) return <p role="alert" className="error">{issues.error.message}</p>;

  return (
    <div className="board-wrap">
      <QuickAdd projectKey={projectKey} />
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      <div className="board">
        {STATUSES.map((col) => {
          const cards = issues.data.filter((i) => i.status === col.id);
          return (
            <section
              key={col.id}
              aria-label={col.label}
              className={`column${over === col.id ? " over" : ""}`}
              onDragOver={(e) => {
                e.preventDefault();
                setOver(col.id);
              }}
              onDragLeave={() => setOver(null)}
              onDrop={(e) => onDrop(e, col.id)}
            >
              <h2>
                {col.label} <span className="count">{cards.length}</span>
              </h2>
              {cards.map((i) => (
                <article
                  key={i.id}
                  className="card"
                  draggable
                  onDragStart={(e) => {
                    e.dataTransfer.setData(DRAG_TYPE, i.key);
                    e.dataTransfer.effectAllowed = "move";
                  }}
                >
                  <span className={`type type-${i.type}`}>{i.type}</span>
                  <p>{i.title}</p>
                  <small>{i.key}</small>
                </article>
              ))}
            </section>
          );
        })}
      </div>
    </div>
  );
}

function QuickAdd({ projectKey }: { projectKey: string }) {
  const qc = useQueryClient();
  const [title, setTitle] = useState("");
  const [type, setType] = useState<IssueType>("task");
  const create = useMutation({
    mutationFn: () => api.createIssue(projectKey, title, type),
    onSuccess: () => {
      setTitle("");
      return qc.invalidateQueries({ queryKey: ["issues", projectKey] });
    },
  });

  function submit(e: FormEvent) {
    e.preventDefault();
    if (title.trim()) create.mutate();
  }

  return (
    <form className="quick-add" onSubmit={submit}>
      <input aria-label="New issue title" placeholder="What needs to be done?" value={title} onChange={(e) => setTitle(e.target.value)} />
      <select aria-label="Type" value={type} onChange={(e) => setType(e.target.value as IssueType)}>
        {ISSUE_TYPES.map((t) => (
          <option key={t} value={t}>
            {t}
          </option>
        ))}
      </select>
      <button type="submit" disabled={create.isPending}>
        Add issue
      </button>
      {create.isError && (
        <span role="alert" className="error">
          {create.error.message}
        </span>
      )}
    </form>
  );
}
