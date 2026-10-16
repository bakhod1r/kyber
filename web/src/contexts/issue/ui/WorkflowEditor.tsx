import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useEffect, useState } from "react";
import { canTransition, issueUseCases, layoutWorkflow, STATUS_CATEGORIES, type StatusCategory, toggleTransition, type Workflow } from "../application";
import { useT } from "../../../shared/i18n/i18n";

/** slug turns a status name into a stable id ("Code Review" → "code_review"). */
export function slug(name: string): string {
  return name.trim().toLowerCase().replace(/[^a-z0-9]+/g, "_").replace(/^_+|_+$/g, "").slice(0, 40) || "status";
}

/** Inserts a status after the last one of its category, so columns stay To Do → In Progress → Done. */
function insertByCategory(wf: Workflow, name: string, category: StatusCategory): Workflow {
  const taken = new Set(wf.statuses.map((s) => s.id));
  let id = slug(name);
  for (let n = 2; taken.has(id); n++) id = `${slug(name)}_${n}`;
  const rank = (c: StatusCategory) => STATUS_CATEGORIES.findIndex((x) => x.id === c);
  let at = wf.statuses.length;
  for (let i = 0; i < wf.statuses.length; i++) {
    if (rank(wf.statuses[i]!.category) > rank(category)) {
      at = i;
      break;
    }
  }
  const statuses = [...wf.statuses];
  statuses.splice(at, 0, { id, name: name.trim(), category });
  return { ...wf, statuses };
}

/** WorkflowEditor lets project admins rename, add and remove statuses (Jira's simplified workflow editor). */
export function WorkflowEditor({ projectKey }: { projectKey: string }) {
  const t = useT();
  const qc = useQueryClient();
  const loaded = useQuery({ queryKey: ["workflow", projectKey], queryFn: () => issueUseCases.workflow(projectKey) });
  const [draft, setDraft] = useState<Workflow | null>(null);
  const [name, setName] = useState("");
  const [category, setCategory] = useState<StatusCategory>("in_progress");
  useEffect(() => {
    if (loaded.data) setDraft(loaded.data);
  }, [loaded.data]);

  const save = useMutation({
    mutationFn: (wf: Workflow) => issueUseCases.setWorkflow(projectKey, wf),
    onSuccess: (wf) => {
      qc.setQueryData(["workflow", projectKey], wf);
      return qc.invalidateQueries({ queryKey: ["issues", projectKey] });
    },
  });

  if (loaded.isPending) return <p className="muted">{t("Loading workflow…")}</p>;
  if (loaded.isError) return <p role="alert" className="error">{loaded.error.message}</p>;
  if (!draft) return null;

  const update = (wf: Workflow) => {
    save.reset();
    setDraft(wf);
  };
  function add(e: FormEvent) {
    e.preventDefault();
    if (!draft || !name.trim()) return;
    update(insertByCategory(draft, name, category));
    setName("");
  }
  function remove(id: string) {
    if (!draft) return;
    const statuses = draft.statuses.filter((s) => s.id !== id);
    update({
      initial: draft.initial === id ? (statuses[0]?.id ?? "") : draft.initial,
      statuses,
      transitions: draft.transitions.filter((t) => t.from !== id && t.to !== id),
    });
  }

  return (
    <section className="workflow-editor" aria-label={t("Workflow")}>
      <h2>{t("Workflow")}</h2>
      <p className="muted">{t("Statuses are board columns. Each belongs to a category that reports and sprints understand.")}</p>
      <ol className="workflow-statuses">
        {draft.statuses.map((s, i) => (
          <li key={s.id}>
            <input
              aria-label={t("Status {n} name", { n: i + 1 })}
              value={s.name}
              maxLength={60}
              onChange={(e) => update({ ...draft, statuses: draft.statuses.map((x) => (x.id === s.id ? { ...x, name: e.target.value } : x)) })}
            />
            <span className={`status status-${s.category}`}>{STATUS_CATEGORIES.find((c) => c.id === s.category)?.label}</span>
            <button type="button" className="ghost danger-ghost" aria-label={t("Remove {name}", { name: s.name })} onClick={() => remove(s.id)} disabled={draft.statuses.length <= 1}>
              {t("Remove")}
            </button>
          </li>
        ))}
      </ol>
      <form className="workflow-add" onSubmit={add}>
        <input aria-label={t("New status name")} placeholder={t("New status")} maxLength={60} value={name} onChange={(e) => setName(e.target.value)} />
        <select aria-label={t("New status category")} value={category} onChange={(e) => setCategory(e.target.value as StatusCategory)}>
          {STATUS_CATEGORIES.map((c) => (
            <option key={c.id} value={c.id}>
              {c.label}
            </option>
          ))}
        </select>
        <button type="submit" className="ghost">{t("Add status")}</button>
      </form>
      <label>
        {t("Initial status")}{" "}
        <select aria-label={t("Initial status")} value={draft.initial} onChange={(e) => update({ ...draft, initial: e.target.value })}>
          {draft.statuses.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </select>
      </label>
      <p className="muted">
        {draft.transitions.length === 0 ? t("Any status can move to any other status.") : t("{n} transitions are configured.", { n: draft.transitions.length })}
      </p>
      <WorkflowDiagram wf={draft} />
      <TransitionMatrix wf={draft} onChange={update} />
      <button type="button" onClick={() => save.mutate(draft)} disabled={save.isPending}>
        {t("Save workflow")}
      </button>
      {save.isSuccess && <p role="status" className="muted">{t("Workflow saved")}</p>}
      {save.isError && (
        <p role="alert" className="error">
          {save.error.message}
        </p>
      )}
    </section>
  );
}

/** WorkflowDiagram draws statuses in category columns with an arrow per allowed transition. */
function WorkflowDiagram({ wf }: { wf: Workflow }) {
  const t = useT();
  const pos = layoutWorkflow(wf);
  const rows = Math.max(1, ...STATUS_CATEGORIES.map((c) => wf.statuses.filter((s) => s.category === c.id).length));
  const edges = wf.statuses.flatMap((a) => wf.statuses.filter((b) => canTransition(wf, a.id, b.id)).map((b) => [a.id, b.id] as const));
  const W = 580, H = 40 + rows * 70;
  return (
    <svg className="wf-diagram" viewBox={`0 0 ${W} ${H}`} role="img" aria-label={t("Workflow diagram")}>
      <defs>
        <marker id="wf-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
          <path d="M0,0 L10,5 L0,10 z" fill="currentColor" />
        </marker>
      </defs>
      {edges.map(([a, b]) => {
        const p = pos.get(a)!, q = pos.get(b)!;
        // Curve each direction to its own side so a ⇄ b shows two arrows.
        const mx = (p.x + q.x) / 2 + (q.y - p.y) * 0.25, my = (p.y + q.y) / 2 - (q.x - p.x) * 0.25;
        return <path key={`${a}>${b}`} className="wf-edge" d={`M${p.x},${p.y} Q${mx},${my} ${q.x},${q.y}`} fill="none" stroke="currentColor" markerEnd="url(#wf-arrow)" />;
      })}
      {wf.statuses.map((s) => {
        const p = pos.get(s.id)!;
        return (
          <g key={s.id} className={`wf-node status-${s.category}`} transform={`translate(${p.x},${p.y})`}>
            <rect x={-70} y={-16} width={140} height={32} rx={8} className={s.id === wf.initial ? "initial" : undefined} />
            <text textAnchor="middle" dominantBaseline="middle">{s.name}</text>
          </g>
        );
      })}
    </svg>
  );
}

/** TransitionMatrix: one checkbox per "from → to" pair (rows: from, columns: to). */
function TransitionMatrix({ wf, onChange }: { wf: Workflow; onChange: (wf: Workflow) => void }) {
  const t = useT();
  return (
    <div className="wf-matrix">
      <table>
        <caption>{t("Allowed transitions")}</caption>
        <thead>
          <tr>
            <th scope="col">{t("From / To")}</th>
            {wf.statuses.map((s) => (
              <th key={s.id} scope="col">{s.name}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {wf.statuses.map((a) => (
            <tr key={a.id}>
              <th scope="row">{a.name}</th>
              {wf.statuses.map((b) => (
                <td key={b.id}>
                  {a.id !== b.id && (
                    <input type="checkbox" aria-label={`${a.name} → ${b.name}`} checked={canTransition(wf, a.id, b.id)} onChange={() => onChange(toggleTransition(wf, a.id, b.id))} />
                  )}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
      {wf.transitions.length > 0 && (
        <button type="button" className="ghost" onClick={() => onChange({ ...wf, transitions: [] })}>
          {t("Allow every transition")}
        </button>
      )}
    </div>
  );
}
