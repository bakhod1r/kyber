import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useEffect, useState } from "react";
import { STATUS_CATEGORIES, type StatusCategory, type Workflow, api } from "../api";

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
  const qc = useQueryClient();
  const loaded = useQuery({ queryKey: ["workflow", projectKey], queryFn: () => api.workflow(projectKey) });
  const [draft, setDraft] = useState<Workflow | null>(null);
  const [name, setName] = useState("");
  const [category, setCategory] = useState<StatusCategory>("in_progress");
  useEffect(() => {
    if (loaded.data) setDraft(loaded.data);
  }, [loaded.data]);

  const save = useMutation({
    mutationFn: (wf: Workflow) => api.setWorkflow(projectKey, wf),
    onSuccess: (wf) => {
      qc.setQueryData(["workflow", projectKey], wf);
      return qc.invalidateQueries({ queryKey: ["issues", projectKey] });
    },
  });

  if (loaded.isPending) return <p className="muted">Loading workflow…</p>;
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
    <section className="workflow-editor" aria-label="Workflow">
      <h2>Workflow</h2>
      <p className="muted">Statuses are board columns. Each belongs to a category that reports and sprints understand.</p>
      <ol className="workflow-statuses">
        {draft.statuses.map((s, i) => (
          <li key={s.id}>
            <input
              aria-label={`Status ${i + 1} name`}
              value={s.name}
              maxLength={60}
              onChange={(e) => update({ ...draft, statuses: draft.statuses.map((x) => (x.id === s.id ? { ...x, name: e.target.value } : x)) })}
            />
            <span className={`status status-${s.category}`}>{STATUS_CATEGORIES.find((c) => c.id === s.category)?.label}</span>
            <button type="button" className="ghost" aria-label={`Remove ${s.name}`} onClick={() => remove(s.id)} disabled={draft.statuses.length <= 1}>
              Remove
            </button>
          </li>
        ))}
      </ol>
      <form className="workflow-add" onSubmit={add}>
        <input aria-label="New status name" placeholder="New status" maxLength={60} value={name} onChange={(e) => setName(e.target.value)} />
        <select aria-label="New status category" value={category} onChange={(e) => setCategory(e.target.value as StatusCategory)}>
          {STATUS_CATEGORIES.map((c) => (
            <option key={c.id} value={c.id}>
              {c.label}
            </option>
          ))}
        </select>
        <button type="submit">Add status</button>
      </form>
      <label>
        Initial status{" "}
        <select aria-label="Initial status" value={draft.initial} onChange={(e) => update({ ...draft, initial: e.target.value })}>
          {draft.statuses.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </select>
      </label>
      <p className="muted">
        {draft.transitions.length === 0 ? "Any status can move to any other status." : `${draft.transitions.length} transitions are configured.`}
      </p>
      <button type="button" onClick={() => save.mutate(draft)} disabled={save.isPending}>
        Save workflow
      </button>
      {save.isSuccess && <p role="status" className="muted">Workflow saved</p>}
      {save.isError && (
        <p role="alert" className="error">
          {save.error.message}
        </p>
      )}
    </section>
  );
}
