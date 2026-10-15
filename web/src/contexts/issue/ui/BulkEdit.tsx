import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { agileUseCases } from "../../agile/application";
import { projectUseCases } from "../../project/application";
import { parseLabels, issueUseCases, type BulkChange, type BulkResult } from "../application";
import { useT } from "../../../shared/i18n/i18n";

const KEEP = "__keep";
const NONE = "__none";

/** BulkEdit: change status, assignee, sprint and labels of the selected issues at once (G9). */
export function BulkEdit({ projectKey, keys, onDone }: { projectKey: string; keys: string[]; onDone: () => void }) {
  const t = useT();
  const qc = useQueryClient();
  const workflow = useQuery({ queryKey: ["workflow", projectKey], queryFn: () => issueUseCases.workflow(projectKey) });
  const members = useQuery({ queryKey: ["members", projectKey], queryFn: () => projectUseCases.members(projectKey) });
  const sprints = useQuery({ queryKey: ["sprints", projectKey], queryFn: () => agileUseCases.sprints(projectKey) });
  const [status, setStatus] = useState(KEEP);
  const [assignee, setAssignee] = useState(KEEP);
  const [sprint, setSprint] = useState(KEEP);
  const [labels, setLabels] = useState("");
  const [failed, setFailed] = useState<{ ok: number; failures: BulkResult[] } | null>(null);
  const change: BulkChange = { keys };
  if (status !== KEEP) change.status = status;
  if (assignee !== KEEP) change.assignee_id = assignee === NONE ? "" : assignee;
  if (sprint !== KEEP) change.sprint_id = sprint === NONE ? "" : sprint;
  const added = parseLabels(labels);
  if (added.length > 0) change.add_labels = added;
  const nothing = Object.keys(change).length === 1;
  const apply = useMutation({
    mutationFn: () => issueUseCases.bulkChange(projectKey, change),
    onSuccess: async (results) => {
      await qc.invalidateQueries();
      const failures = results.filter((r) => !r.ok);
      if (failures.length === 0) return onDone();
      setFailed({ ok: results.length - failures.length, failures });
    },
  });

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    setFailed(null);
    apply.mutate();
  }

  return (
    <form className="panel bulk-edit" onSubmit={onSubmit} aria-label={t("Bulk change")}>
      <strong>{keys.length === 1 ? t("1 issue selected") : t("{n} issues selected", { n: keys.length })}</strong>
      <label>
        {t("Status")}
        <select value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value={KEEP}>{t("No change")}</option>
          {workflow.data?.statuses.map((s) => (
            <option key={s.id} value={s.id}>
              {s.name}
            </option>
          ))}
        </select>
      </label>
      <label>
        {t("Assignee")}
        <select value={assignee} onChange={(e) => setAssignee(e.target.value)}>
          <option value={KEEP}>{t("No change")}</option>
          <option value={NONE}>{t("Unassigned")}</option>
          {members.data?.map((m) => (
            <option key={m.user_id} value={m.user_id}>
              {m.name || m.email}
            </option>
          ))}
        </select>
      </label>
      <label>
        {t("Sprint")}
        <select value={sprint} onChange={(e) => setSprint(e.target.value)}>
          <option value={KEEP}>{t("No change")}</option>
          <option value={NONE}>{t("Backlog")}</option>
          {sprints.data
            ?.filter((s) => s.state !== "closed")
            .map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
        </select>
      </label>
      <label>
        {t("Add labels")}
        <input value={labels} onChange={(e) => setLabels(e.target.value)} placeholder="q4 urgent" />
      </label>
      <div className="row">
        <button type="submit" disabled={nothing || apply.isPending}>
          {keys.length === 1 ? t("Apply to 1 issue") : t("Apply to {n} issues", { n: keys.length })}
        </button>
        <button type="button" className="ghost" onClick={onDone}>
          {t("Cancel")}
        </button>
      </div>
      {apply.isError && (
        <p role="alert" className="error">
          {apply.error.message}
        </p>
      )}
      {failed && (
        <div role="status" className="bulk-result">
          <p>{t("{ok} changed, {failed} failed", { ok: failed.ok, failed: failed.failures.length })}</p>
          <ul>
            {failed.failures.map((f) => (
              <li key={f.key}>
                {f.key}: {f.error}
              </li>
            ))}
          </ul>
        </div>
      )}
    </form>
  );
}
