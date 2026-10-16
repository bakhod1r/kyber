import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type DragEvent, type FormEvent, useState } from "react";
import { Link } from "react-router-dom";
import { agileUseCases, type BoardColumn, type BoardFilter, boardFilterActive, filterBoard, NO_BOARD_FILTER, SWIMLANE_MODES, swimlanes, type SwimlaneMode, toggle, type Version } from "../application";
import { issueUseCases, DEFAULT_WORKFLOW, ISSUE_TYPES, type Issue, type IssueType, type Status } from "../../issue/application";
import { projectUseCases, type Member } from "../../project/application";
import { announceIssueDone } from "../../../shared/events";
import { useLabel, useT } from "../../../shared/i18n/i18n";

const DRAG_TYPE = "application/x-kyber-issue";

/** The swimlane choice is remembered per project in this browser (best effort). */
function storedMode(projectKey: string): SwimlaneMode {
  try {
    const v = localStorage.getItem(`kyber.swimlanes.${projectKey}`);
    return SWIMLANE_MODES.find((m) => m === v) ?? "none";
  } catch {
    return "none";
  }
}

function storeMode(projectKey: string, mode: SwimlaneMode) {
  try {
    localStorage.setItem(`kyber.swimlanes.${projectKey}`, mode);
  } catch {
    // private mode or blocked storage: the choice lasts until reload
  }
}

export function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  return ((parts[0]?.[0] ?? "") + (parts.length > 1 ? (parts[parts.length - 1]?.[0] ?? "") : "")).toUpperCase() || "?";
}

export function Board({
  projectKey,
  onOpen,
  visibleKeys,
}: {
  projectKey: string;
  onOpen?: (issueKey: string) => void;
  /** Quick filters (composed by the app): only these issues show; null shows all. */
  visibleKeys?: Set<string> | null;
}) {
  const t = useT();
  const label = useLabel();
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
  const selectTo = columns[1]?.statuses[0];
  const byId = new Map<string, Member>((members.data ?? []).map((m) => [m.user_id, m]));
  const [error, setError] = useState<string | null>(null);
  const [over, setOver] = useState<string | null>(null);
  const [mode, setMode] = useState<SwimlaneMode>(() => storedMode(projectKey));
  const [filter, setFilter] = useState<BoardFilter>(NO_BOARD_FILTER);
  // Versions load when the Version filter is first opened, not with the board.
  const [wantVersions, setWantVersions] = useState(false);
  const versions = useQuery({ queryKey: ["versions", projectKey], queryFn: () => agileUseCases.versions(projectKey), enabled: wantVersions });
  const passing = new Set(filterBoard(issues.data ?? [], filter).map((i) => i.key));
  /** Quick filters (from the app) and the toolbar's filters both apply. */
  const isShown = (i: Issue): boolean => (!visibleKeys || visibleKeys.has(i.key)) && passing.has(i.key);
  const waiting = kanbanBacklog ? (issues.data ?? []).filter((i) => columns[0]?.statuses.includes(i.status) && isShown(i)) : [];
  const complete = useMutation({
    mutationFn: (id: string) => agileUseCases.completeSprint(id),
    onMutate: () => setError(null),
    onError: (err) => setError(err.message),
    onSettled: () => qc.invalidateQueries({ queryKey: ["sprints", projectKey] }),
  });
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
    moveTo(e.dataTransfer.getData(DRAG_TYPE), col);
  }

  // The card's Move menu does the same without a mouse (keyboard, touch screens).
  function moveTo(key: string, col: BoardColumn | undefined) {
    const issue = issues.data?.find((i) => i.key === key);
    const to = col?.statuses[0];
    if (issue && col && to && !col.statuses.includes(issue.status)) move.mutate({ key, to });
  }

  const laneLabel = (id: string): string => {
    if (mode === "assignee") return id ? (byId.get(id)?.name ?? id) : t("Unassigned");
    if (!id) return t("No epic");
    const epic = issues.data?.find((i) => i.key === id);
    return epic ? `${id} ${epic.title}` : id;
  };

  if (sprints.isPending || issues.isPending || boards.isPending || workflow.isPending) return <p className="muted">{t("Loading board…")}</p>;
  if (issues.isError) return <p role="alert" className="error">{issues.error.message}</p>;

  const renderColumns = (laneIssues: Issue[], lane?: string) =>
    boardColumns.map((col) => {
      const shown = (i: Issue) => col.statuses.includes(i.status) && isShown(i);
      const cards = laneIssues.filter(shown);
      const total = laneIssues === issues.data ? cards.length : issues.data.filter(shown).length; // WIP counts the whole column
      const wip = col.max > 0 && total > col.max ? " wip-over" : col.min > 0 && total < col.min ? " wip-under" : "";
      return (
        <section
          key={col.name}
          aria-label={lane === undefined ? label(col.name) : `${label(col.name)} · ${lane}`}
          className={`column${over === col.name ? " over" : ""}${wip}`}
          onDragOver={(e) => {
            e.preventDefault();
            setOver(col.name);
          }}
          onDragLeave={() => setOver(null)}
          onDrop={(e) => onDrop(e, col)}
        >
          <h2>
            {label(col.name)}{" "}
            <span className="count" title={col.max > 0 ? t("Issues / WIP limit") : undefined}>
              {col.max > 0 ? `${total} / ${col.max}` : cards.length}
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
                <span className={`prio prio-${i.priority}`} aria-label={t("Priority: {priority}", { priority: i.priority })} title={t("Priority: {priority}", { priority: i.priority })} />
              </div>
              <button type="button" className="card-open" onClick={() => onOpen?.(i.key)}>
                {i.title}
              </button>
              <div className="card-foot">
                <small>{i.key}</small>
                <select
                  className="card-move"
                  aria-label={t("Move {key}", { key: i.key })}
                  value={col.name}
                  onChange={(e) => moveTo(i.key, boardColumns.find((c) => c.name === e.target.value))}
                >
                  {boardColumns.map((c) => (
                    <option key={c.name} value={c.name}>
                      {label(c.name)}
                    </option>
                  ))}
                </select>
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
    });

  return (
    <div className="board-wrap">
      {active && (
        <div className="sprint-banner">
          <p>
            <strong>{active.name}</strong>
            {active.goal && <span> — {active.goal}</span>}
          </p>
          <button type="button" disabled={complete.isPending} onClick={() => complete.mutate(active.id)} title={active.name}>
            {t("Complete sprint")}
          </button>
        </div>
      )}
      {boards.data && boards.data.length > 1 && (
        <label className="board-picker">
          {t("Board")}{" "}
          <select aria-label={t("Board")} value={board?.id} onChange={(e) => setBoardId(e.target.value)}>
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
          {t("Kanban backlog")}
        </label>
      )}
      <QuickAdd projectKey={projectKey} sprintId={active?.id} />
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      <BoardToolbar
        projectKey={projectKey}
        issues={issues.data}
        members={members.data ?? []}
        versions={versions.data ?? []}
        onVersionsOpen={() => setWantVersions(true)}
        filter={filter}
        onFilter={setFilter}
        mode={mode}
        onMode={(m) => {
          setMode(m);
          storeMode(projectKey, m);
        }}
      />
      {mode === "none" ? (
        <div className="board">{renderColumns(issues.data)}</div>
      ) : (
        swimlanes(issues.data, mode).map((l) => (
          <section key={l.id || "none"} className="swimlane" aria-label={laneLabel(l.id)}>
            <h3>
              {laneLabel(l.id)} <span className="count">{l.items.filter(isShown).length}</span>
            </h3>
            <div className="board">{renderColumns(l.items, laneLabel(l.id))}</div>
          </section>
        ))
      )}
      {kanbanBacklog && (
        <section aria-label={t("Kanban backlog")} className="kanban-backlog panel">
          <h2>
            {t("Backlog")} <span className="count">{waiting.length}</span>
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
                  aria-label={t("Select {key} for development", { key: i.key })}
                  onClick={() => selectTo && move.mutate({ key: i.key, to: selectTo })}
                >
                  {t("Select for development")}
                </button>
              </li>
            ))}
          </ul>
        </section>
      )}
    </div>
  );
}

/** BoardToolbar: Jira's board header — search, people, version/epic/type filters, grouping and
 * sprint completion. */
function BoardToolbar({
  projectKey,
  issues,
  members,
  versions,
  onVersionsOpen,
  filter,
  onFilter,
  mode,
  onMode,
}: {
  projectKey: string;
  issues: Issue[];
  members: Member[];
  versions: Version[];
  onVersionsOpen: () => void;
  filter: BoardFilter;
  onFilter: (f: BoardFilter) => void;
  mode: SwimlaneMode;
  onMode: (m: SwimlaneMode) => void;
}) {
  const t = useT();
  const set = (patch: Partial<BoardFilter>) => onFilter({ ...filter, ...patch });
  const epics = issues.filter((i) => i.type === "epic");
  const types = [...new Set(issues.map((i) => i.type))];
  const labels = [...new Set(issues.flatMap((i) => i.labels ?? []))].sort();
  const enc = encodeURIComponent(projectKey);
  return (
    <div className="board-toolbar" role="toolbar" aria-label={t("Board filters")}>
      <input
        type="search"
        className="board-search"
        aria-label={t("Search board")}
        placeholder={t("Search board")}
        value={filter.text}
        onChange={(e) => set({ text: e.target.value })}
      />
      <div className="avatar-filter" role="group" aria-label={t("Assignees")}>
        {members.map((m) => (
          <button
            key={m.user_id}
            type="button"
            className="avatar"
            title={m.name}
            aria-label={m.name}
            aria-pressed={filter.assignees.includes(m.user_id)}
            onClick={() => set({ assignees: toggle(filter.assignees, m.user_id) })}
          >
            {initials(m.name)}
          </button>
        ))}
        <button
          type="button"
          className="avatar unassigned"
          title={t("Unassigned")}
          aria-label={t("Unassigned")}
          aria-pressed={filter.assignees.includes("")}
          onClick={() => set({ assignees: toggle(filter.assignees, "") })}
        >
          ?
        </button>
      </div>
      <MultiPick label={t("Version")} aria={t("Filter by {name}", { name: t("Version") })} onOpen={onVersionsOpen} options={versions.map((v) => ({ id: v.id, label: v.name }))} value={filter.versions} onChange={(versions) => set({ versions })} />
      <MultiPick
        label={t("Epic")}
        aria={t("Filter by {name}", { name: t("Epic") })}
        options={[...epics.map((e) => ({ id: e.key, label: `${e.key} ${e.title}` })), { id: "", label: t("No epic") }]}
        value={filter.epics}
        onChange={(epics) => set({ epics })}
      />
      <MultiPick label={t("Type")} aria={t("Filter by kind")} options={types.map((ty) => ({ id: ty, label: ty }))} value={filter.types} onChange={(types) => set({ types })} />
      <MultiPick label={t("Label")} aria={t("Filter by {name}", { name: t("Label") })} options={labels.map((l) => ({ id: l, label: l }))} value={filter.labels} onChange={(labels) => set({ labels })} />
      {boardFilterActive(filter) && (
        <button type="button" className="ghost" onClick={() => onFilter(NO_BOARD_FILTER)}>
          {t("Clear filters")}
        </button>
      )}
      <span className="toolbar-spacer" />
      <label className="board-picker">
        {t("Group")}{" "}
        <select aria-label={t("Group")} value={mode} onChange={(e) => onMode(SWIMLANE_MODES.find((x) => x === e.target.value) ?? "none")}>
          <option value="none">{t("None")}</option>
          <option value="assignee">{t("Assignee")}</option>
          <option value="epic">{t("Epic")}</option>
        </select>
      </label>
      <Link className="icon-link" to={`/projects/${enc}/reports`} title={t("Reports")} aria-label={t("Board insights")}>
        📈
      </Link>
      <Link className="icon-link" to={`/projects/${enc}/board-settings`} title={t("Board settings")} aria-label={t("Configure board")}>
        ⚙
      </Link>
    </div>
  );
}

/** MultiPick: a dropdown of checkboxes; the button shows how many values are picked. */
function MultiPick({
  label,
  aria,
  options,
  value,
  onChange,
  onOpen,
}: {
  label: string;
  aria: string;
  options: { id: string; label: string }[];
  value: string[];
  onChange: (v: string[]) => void;
  onOpen?: () => void;
}) {
  return (
    <details className="multi-pick" onToggle={(e) => e.currentTarget.open && onOpen?.()}>
      <summary aria-label={aria}>
        {label}
        {value.length > 0 && <span className="count"> {value.length}</span>} ▾
      </summary>
      <div className="multi-pick-menu" role="group" aria-label={aria}>
        {options.length === 0 && <span className="muted">—</span>}
        {options.map((o) => (
          <label key={o.id || "none"} className="check">
            <input type="checkbox" checked={value.includes(o.id)} onChange={() => onChange(toggle(value, o.id))} />
            {o.label}
          </label>
        ))}
      </div>
    </details>
  );
}

// QuickAdd creates an issue; during an active sprint it joins that sprint (as in Jira),
// otherwise it would land in the backlog and vanish from the sprint board.
function QuickAdd({ projectKey, sprintId }: { projectKey: string; sprintId?: string }) {
  const t = useT();
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
      <input aria-label={t("New issue title")} placeholder={t("What needs to be done?")} value={title} onChange={(e) => setTitle(e.target.value)} />
      <select aria-label={t("Type")} value={type} onChange={(e) => setType(e.target.value as IssueType)}>
        {ISSUE_TYPES.map((t) => (
          <option key={t} value={t}>
            {t}
          </option>
        ))}
      </select>
      <button type="submit" disabled={create.isPending}>
        {t("Add issue")}
      </button>
      {create.isError && (
        <span role="alert" className="error">
          {create.error.message}
        </span>
      )}
    </form>
  );
}
