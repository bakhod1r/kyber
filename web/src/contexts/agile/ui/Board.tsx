import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type DragEvent, type FormEvent, useState } from "react";
import { agileUseCases, type BoardColumn } from "../application";
import { issueUseCases, DEFAULT_WORKFLOW, ISSUE_TYPES, type Issue, type IssueType, type Status } from "../../issue/application";
import { projectUseCases, type Member } from "../../project/application";
import { announceIssueDone } from "../../../shared/events";

const DRAG_TYPE = "application/x-kyber-issue";

export function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  return ((parts[0]?.[0] ?? "") + (parts.length > 1 ? (parts[parts.length - 1]?.[0] ?? "") : "")).toUpperCase() || "?";
}

export function Board({ projectKey, onOpen }: { projectKey: string; onOpen?: (issueKey: string) => void }) {
  const qc = useQueryClient();
  // With an active sprint the board shows only that sprint (Scrum); otherwise every issue (Kanban).
  const sprints = useQuery({ queryKey: ["sprints", projectKey], queryFn: () => agileUseCases.sprints(projectKey) });
  const active = sprints.data?.find((s) => s.state === "active");
  const queryKey = ["issues", projectKey, active?.id ?? "all"];
  const issues = useQuery({
    queryKey,
    queryFn: () => issueUseCases.issues(projectKey, active?.id),
    enabled: !sprints.isPending,
  });
  const members = useQuery({ queryKey: ["members", projectKey], queryFn: () => projectUseCases.members(projectKey) });
  // Columns come from the selected board (S44); without boards, one column per workflow status.
  const boards = useQuery({ queryKey: ["boards", projectKey], queryFn: () => agileUseCases.boards(projectKey) });
  const workflow = useQuery({ queryKey: ["workflow", projectKey], queryFn: () => issueUseCases.workflow(projectKey) });
  const [boardId, setBoardId] = useState<string | null>(null);
  const board = boards.data?.find((b) => b.id === boardId) ?? boards.data?.[0];
  const columns: BoardColumn[] =
    board?.columns ?? (workflow.data ?? DEFAULT_WORKFLOW).statuses.map((st) => ({ name: st.name, statuses: [st.id], min: 0, max: 0 }));
  // Kanban backlog (S48): the first column's issues wait in a list below instead of on the board.
  const kanbanBacklog = !active && !!board?.kanban_backlog && columns.length > 1;
  const boardColumns = kanbanBacklog ? columns.slice(1) : columns;
  const waiting = kanbanBacklog ? (issues.data ?? []).filter((i) => columns[0]?.statuses.includes(i.status)) : [];
  const selectTo = columns[1]?.statuses[0];
  const byId = new Map<string, Member>((members.data ?? []).map((m) => [m.user_id, m]));
  const [error, setError] = useState<string | null>(null);
  const [over, setOver] = useState<string | null>(null);
  const toggleBacklog = useMutation({
    mutationFn: (on: boolean) => agileUseCases.updateBoard(board!, { kanban_backlog: on }),
    onMutate: () => setError(null),
    onError: (err) => setError(err.message),
    onSettled: () => qc.invalidateQueries({ queryKey: ["boards", projectKey] }),
  });

  const move = useMutation({
    mutationFn: ({ key, to }: { key: string; to: Status }) => issueUseCases.transition(key, to),
    // Optimistic: move the card now, roll back on rejection.
    onMutate: async ({ key, to }) => {
      setError(null);
      await qc.cancelQueries({ queryKey });
      const previous = qc.getQueryData<Issue[]>(queryKey);
      qc.setQueryData<Issue[]>(queryKey, (list) => list?.map((i) => (i.key === key ? { ...i, status: to } : i)));
      return { previous };
    },
    onSuccess: (_issue, { key, to }) => {
      if ((workflow.data ?? DEFAULT_WORKFLOW).statuses.find((st) => st.id === to)?.category === "done") announceIssueDone(key);
    },
    onError: (err, _vars, ctx) => {
      qc.setQueryData(queryKey, ctx?.previous);
      setError(err.message);
    },
    onSettled: () => qc.invalidateQueries({ queryKey }),
  });

  // Dropping on a column moves the issue to the column's first status (Jira behaviour).
  function onDrop(e: DragEvent, col: BoardColumn) {
    e.preventDefault();
    setOver(null);
    const key = e.dataTransfer.getData(DRAG_TYPE);
    const issue = issues.data?.find((i) => i.key === key);
    const to = col.statuses[0];
    if (issue && to && !col.statuses.includes(issue.status)) move.mutate({ key, to });
  }

  if (sprints.isPending || issues.isPending || boards.isPending || workflow.isPending) return <p className="muted">Loading board…</p>;
  if (issues.isError) return <p role="alert" className="error">{issues.error.message}</p>;

  return (
    <div className="board-wrap">
      {active && (
        <p className="sprint-banner">
          <strong>{active.name}</strong>
          {active.goal && <span> — {active.goal}</span>}
        </p>
      )}
      {boards.data && boards.data.length > 1 && (
        <label className="board-picker">
          Board{" "}
          <select aria-label="Board" value={board?.id} onChange={(e) => setBoardId(e.target.value)}>
            {boards.data.map((b) => (
              <option key={b.id} value={b.id}>
                {b.name}
              </option>
            ))}
          </select>
        </label>
      )}
      {board?.kind === "kanban" && columns.length > 1 && !active && (
        <label className="check">
          <input
            type="checkbox"
            checked={!!board.kanban_backlog}
            disabled={toggleBacklog.isPending}
            onChange={(e) => toggleBacklog.mutate(e.target.checked)}
          />
          Kanban backlog
        </label>
      )}
      <QuickAdd projectKey={projectKey} sprintId={active?.id} />
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      <div className="board">
        {boardColumns.map((col) => {
          const cards = issues.data.filter((i) => col.statuses.includes(i.status));
          const wip = col.max > 0 && cards.length > col.max ? " wip-over" : col.min > 0 && cards.length < col.min ? " wip-under" : "";
          return (
            <section
              key={col.name}
              aria-label={col.name}
              className={`column${over === col.name ? " over" : ""}${wip}`}
              onDragOver={(e) => {
                e.preventDefault();
                setOver(col.name);
              }}
              onDragLeave={() => setOver(null)}
              onDrop={(e) => onDrop(e, col)}
            >
              <h2>
                {col.name}{" "}
                <span className="count" title={col.max > 0 ? "Issues / WIP limit" : undefined}>
                  {col.max > 0 ? `${cards.length} / ${col.max}` : cards.length}
                </span>
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
                  <div className="card-head">
                    <span className={`type type-${i.type}`}>{i.type}</span>
                    <span className={`prio prio-${i.priority}`} aria-label={`Priority: ${i.priority}`} title={`Priority: ${i.priority}`} />
                  </div>
                  <button type="button" className="card-open" onClick={() => onOpen?.(i.key)}>
                    {i.title}
                  </button>
                  <div className="card-foot">
                    <small>{i.key}</small>
                    {i.assignee_id && byId.get(i.assignee_id) && (
                      <span className="avatar" title={byId.get(i.assignee_id)?.name}>
                        {initials(byId.get(i.assignee_id)?.name ?? "")}
                      </span>
                    )}
                  </div>
                </article>
              ))}
            </section>
          );
        })}
      </div>
      {kanbanBacklog && (
        <section aria-label="Kanban backlog" className="kanban-backlog panel">
          <h2>
            Backlog <span className="count">{waiting.length}</span>
          </h2>
          <ul className="child-list">
            {waiting.map((i) => (
              <li key={i.id}>
                <span className="key">{i.key}</span>
                <button type="button" className="card-open" onClick={() => onOpen?.(i.key)}>
                  {i.title}
                </button>
                <button
                  type="button"
                  className="ghost"
                  aria-label={`Select ${i.key} for development`}
                  onClick={() => selectTo && move.mutate({ key: i.key, to: selectTo })}
                >
                  Select for development
                </button>
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}

// QuickAdd creates an issue; during an active sprint it joins that sprint (as in Jira),
// otherwise it would land in the backlog and vanish from the sprint board.
function QuickAdd({ projectKey, sprintId }: { projectKey: string; sprintId?: string }) {
  const qc = useQueryClient();
  const [title, setTitle] = useState("");
  const [type, setType] = useState<IssueType>("task");
  const create = useMutation({
    mutationFn: () => issueUseCases.createIssueIn(projectKey, title, type, sprintId),
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
