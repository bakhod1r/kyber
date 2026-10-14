import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { issueUseCases } from "../../issue/application";
import {
  addColumn,
  agileUseCases,
  assignStatus,
  type BoardColumn,
  boardProblems,
  type BoardView,
  columnOf,
  moveColumn,
} from "../application";
import { useT } from "../../../shared/i18n/i18n";

type Draft = { name: string; kind: BoardView["kind"]; columns: BoardColumn[]; backlog: boolean };

const toDraft = (b: BoardView): Draft => ({ name: b.name, kind: b.kind, columns: b.columns, backlog: !!b.kanban_backlog });

/** BoardSettings: Jira's board configuration — columns, the statuses they show and WIP limits. */
export function BoardSettings({ projectKey }: { projectKey: string }) {
  const t = useT();
  const qc = useQueryClient();
  const key = ["boards", projectKey];
  const boards = useQuery({ queryKey: key, queryFn: () => agileUseCases.boards(projectKey) });
  const workflow = useQuery({ queryKey: ["workflow", projectKey], queryFn: () => issueUseCases.workflow(projectKey) });
  const [boardId, setBoardId] = useState<string | null>(null);
  const board = boards.data?.find((b) => b.id === boardId) ?? boards.data?.[0];
  const [draft, setDraft] = useState<Draft | null>(null);
  const [notice, setNotice] = useState<{ kind: "ok" | "error"; text: string } | null>(null);
  const [newName, setNewName] = useState("");
  useEffect(() => {
    if (board) setDraft(toDraft(board));
  }, [board]);

  const done = (text: string, select?: (b: BoardView) => string | null) => ({
    onSuccess: (b: BoardView | undefined) => {
      setNotice({ kind: "ok" as const, text });
      if (select && b) setBoardId(select(b));
      return qc.invalidateQueries({ queryKey: key });
    },
    onError: (err: Error) => setNotice({ kind: "error" as const, text: err.message }),
  });
  const save = useMutation({
    mutationFn: (d: Draft) =>
      agileUseCases.updateBoard(board!, { name: d.name.trim(), kind: d.kind, columns: d.columns, kanban_backlog: d.kind === "kanban" && d.backlog }),
    ...done(t("Board saved")),
  });
  const create = useMutation({
    mutationFn: () => agileUseCases.createBoard(projectKey, newName.trim(), "kanban"),
    ...done(t("Board created"), (b) => {
      setNewName("");
      return b.id;
    }),
  });
  const remove = useMutation({ mutationFn: () => agileUseCases.deleteBoard(board!.id), ...done(t("Board deleted"), () => null) });

  if (boards.isPending || workflow.isPending || !draft || !board) return <p className="muted">{t("Loading…")}</p>;
  const statuses = workflow.data?.statuses ?? [];
  const problems = boardProblems(draft.name, draft.columns);
  const setCols = (columns: BoardColumn[]) => setDraft({ ...draft, columns });
  const setCol = (i: number, patch: Partial<BoardColumn>) => setCols(draft.columns.map((c, j) => (j === i ? { ...c, ...patch } : c)));
  const limit = (v: string) => Math.max(0, Math.floor(Number(v) || 0));

  return (
    <section className="panel board-settings" aria-label={t("Board settings")}>
      <h2>{t("Board settings")}</h2>
      <div className="row">
        {boards.data!.length > 1 && (
          <label>
            {t("Board")}
            <select value={board.id} onChange={(e) => setBoardId(e.target.value)}>
              {boards.data!.map((b) => (
                <option key={b.id} value={b.id}>
                  {b.name}
                </option>
              ))}
            </select>
          </label>
        )}
        <label>
          {t("Board name")}
          <input value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} maxLength={60} />
        </label>
        <label>
          {t("Kind")}
          <select value={draft.kind} onChange={(e) => setDraft({ ...draft, kind: e.target.value as BoardView["kind"] })}>
            <option value="kanban">{t("Kanban")}</option>
            <option value="scrum">{t("Scrum")}</option>
          </select>
        </label>
        {draft.kind === "kanban" && (
          <label className="check">
            <input type="checkbox" checked={draft.backlog} onChange={(e) => setDraft({ ...draft, backlog: e.target.checked })} />
            {t("Kanban backlog (first column)")}
          </label>
        )}
      </div>

      <h3>{t("Columns")}</h3>
      <ol className="columns-editor">
        {draft.columns.map((c, i) => (
          <li key={i} className="row">
            <label>
              {t("Column {n} name", { n: i + 1 })}
              <input value={c.name} onChange={(e) => setCol(i, { name: e.target.value })} maxLength={60} />
            </label>
            <label className="narrow">
              {t("{name} min", { name: c.name })}
              <input type="number" min={0} value={c.min} onChange={(e) => setCol(i, { min: limit(e.target.value) })} />
            </label>
            <label className="narrow">
              {t("{name} max", { name: c.name })}
              <input type="number" min={0} value={c.max} onChange={(e) => setCol(i, { max: limit(e.target.value) })} />
            </label>
            <button type="button" className="ghost" aria-label={t("Move {name} left", { name: c.name })} disabled={i === 0} onClick={() => setCols(moveColumn(draft.columns, i, -1))}>
              ←
            </button>
            <button
              type="button"
              className="ghost"
              aria-label={t("Move {name} right", { name: c.name })}
              disabled={i === draft.columns.length - 1}
              onClick={() => setCols(moveColumn(draft.columns, i, 1))}
            >
              →
            </button>
            <button type="button" className="ghost" aria-label={t("Remove column {name}", { name: c.name })} onClick={() => setCols(draft.columns.filter((_, j) => j !== i))}>
              {t("Remove")}
            </button>
          </li>
        ))}
      </ol>
      <button type="button" className="ghost" onClick={() => setCols(addColumn(draft.columns))}>
        {t("Add column")}
      </button>

      <h3>{t("Statuses")}</h3>
      <table className="versions-table">
        <tbody>
          {statuses.map((st) => (
            <tr key={st.id}>
              <th scope="row">{st.name}</th>
              <td>
                <select
                  aria-label={t("Column for {name}", { name: st.name })}
                  value={columnOf(draft.columns, st.id)}
                  onChange={(e) => setCols(assignStatus(draft.columns, st.id, Number(e.target.value)))}
                >
                  <option value={-1}>{t("Not on the board")}</option>
                  {draft.columns.map((c, i) => (
                    <option key={i} value={i}>
                      {c.name || t("Column {n}", { n: i + 1 })}
                    </option>
                  ))}
                </select>
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {problems.length > 0 && (
        <ul className="error" aria-label={t("Problems")}>
          {problems.map((p) => (
            <li key={p}>{p}</li>
          ))}
        </ul>
      )}
      <div className="row end">
        {notice && (
          <span role={notice.kind === "error" ? "alert" : "status"} className={notice.kind === "error" ? "error" : "muted"}>
            {notice.text}
          </span>
        )}
        <button type="button" className="ghost" onClick={() => setDraft(toDraft(board))}>
          {t("Discard changes")}
        </button>
        <button type="button" disabled={save.isPending || problems.length > 0} onClick={() => save.mutate(draft)}>
          {t("Save board")}
        </button>
      </div>

      <h3>{t("Boards")}</h3>
      <div className="row">
        <label>
          {t("New board name")}
          <input value={newName} onChange={(e) => setNewName(e.target.value)} maxLength={60} />
        </label>
        <button type="button" disabled={create.isPending || !newName.trim()} onClick={() => create.mutate()}>
          {t("Create board")}
        </button>
        <button type="button" className="ghost" disabled={remove.isPending} onClick={() => remove.mutate()}>
          {t("Delete {name}", { name: board.name })}
        </button>
      </div>
    </section>
  );
}
