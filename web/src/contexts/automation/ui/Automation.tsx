import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { issueUseCases } from "../../issue/application";
import { formatDateTime } from "../../../shared/i18n/labels";
import { type MessageKey, msg, useT } from "../../../shared/i18n/i18n";
import { ACTION_KINDS, type Action, type ActionKind, automationUseCases, type RuleInput, ruleReady, type Trigger, TRIGGERS } from "../application";

const TRIGGER_NAMES: Record<Trigger, MessageKey> = {
  "issue.created": msg("An issue is created"),
  "issue.transitioned": msg("An issue changes status"),
  "comment.added": msg("Someone comments"),
  "issue.assigned": msg("An issue is assigned"),
};
const ACTION_NAMES: Record<ActionKind, MessageKey> = {
  transition: msg("Move to status"),
  assign: msg("Assign to (user id, reporter or unassigned)"),
  add_label: msg("Add label"),
  comment: msg("Add comment"),
};

const empty = (): RuleInput => ({ name: "", trigger: "issue.created", to_status: "", condition: "", actions: [{ kind: "comment", value: "" }] });

/** Automation: a project's rules — when something happens to an issue that matches a query, act on it (G8). */
export function Automation({ projectKey }: { projectKey: string }) {
  const t = useT();
  const qc = useQueryClient();
  const rules = useQuery({ queryKey: ["automation", projectKey, "rules"], queryFn: () => automationUseCases.rules(projectKey) });
  const runs = useQuery({ queryKey: ["automation", projectKey, "runs"], queryFn: () => automationUseCases.runs(projectKey) });
  const workflow = useQuery({ queryKey: ["workflow", projectKey], queryFn: () => issueUseCases.workflow(projectKey) });
  const [draft, setDraft] = useState<RuleInput>(empty);
  const refresh = () => qc.invalidateQueries({ queryKey: ["automation", projectKey] });
  const create = useMutation({
    mutationFn: () => automationUseCases.createRule(projectKey, draft),
    onSuccess: () => {
      setDraft(empty());
      return refresh();
    },
  });
  const toggle = useMutation({ mutationFn: (v: { id: string; on: boolean }) => automationUseCases.updateRule(rules.data!.find((r) => r.id === v.id)!, v.on), onSuccess: refresh });
  const remove = useMutation({ mutationFn: automationUseCases.deleteRule, onSuccess: refresh });
  const names = new Map(rules.data?.map((r) => [r.id, r.name]));
  const setAction = (i: number, a: Partial<Action>) => setDraft((d) => ({ ...d, actions: d.actions.map((x, j) => (j === i ? { ...x, ...a } : x)) }));

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    create.mutate();
  }

  return (
    <section className="panel automation" aria-label={t("Automation")}>
      <h2>{t("Automation")}</h2>
      <p className="muted">{t("Rules act on issues for you. They run as the person who created them, and react only to changes people make.")}</p>
      {rules.data?.length === 0 && <p className="muted">{t("No rules yet.")}</p>}
      <ul className="rules">
        {rules.data?.map((r) => (
          <li key={r.id} aria-label={r.name} className={r.enabled ? "" : "off"}>
            <div>
              <strong>{r.name}</strong>
              <small className="muted">
                {" "}
                {t(TRIGGER_NAMES[r.trigger])}
                {r.to_status && ` → ${workflow.data?.statuses.find((s) => s.id === r.to_status)?.name ?? r.to_status}`}
                {r.condition && ` · ${r.condition}`} · {r.actions.map((a) => `${t(ACTION_NAMES[a.kind]).replace(/\s*\(.*\)$/, "")}: ${a.value}`).join("; ")}
              </small>
            </div>
            <label className="check">
              <input type="checkbox" checked={r.enabled} disabled={toggle.isPending} onChange={(e) => toggle.mutate({ id: r.id, on: e.target.checked })} />
              {t("Enabled")}
            </label>
            <button type="button" className="ghost" aria-label={t("Delete rule {name}", { name: r.name })} onClick={() => remove.mutate(r.id)}>
              ✕
            </button>
          </li>
        ))}
      </ul>
      {(toggle.isError || remove.isError) && (
        <p role="alert" className="error">
          {(toggle.error ?? remove.error)?.message}
        </p>
      )}

      <form className="rule-form" onSubmit={onSubmit}>
        <h3>{t("New rule")}</h3>
        <label>
          {t("Rule name")}
          <input value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} required maxLength={100} />
        </label>
        <label>
          {t("When")}
          <select value={draft.trigger} onChange={(e) => setDraft({ ...draft, trigger: e.target.value as Trigger, to_status: "" })}>
            {TRIGGERS.map((tr) => (
              <option key={tr} value={tr}>
                {t(TRIGGER_NAMES[tr])}
              </option>
            ))}
          </select>
        </label>
        {draft.trigger === "issue.transitioned" && (
          <label>
            {t("To status")}
            <select value={draft.to_status} onChange={(e) => setDraft({ ...draft, to_status: e.target.value })}>
              <option value="">{t("Any status")}</option>
              {workflow.data?.statuses.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </select>
          </label>
        )}
        <label>
          {t("If the issue matches (JQL, optional)")}
          <input value={draft.condition} onChange={(e) => setDraft({ ...draft, condition: e.target.value })} placeholder="type = bug AND priority = highest" />
        </label>
        <fieldset className="rule-actions">
          <legend>{t("Then")}</legend>
          {draft.actions.map((a, i) => (
            <div key={i} className="rule-action">
              <select aria-label={t("Action {n}", { n: i + 1 })} value={a.kind} onChange={(e) => setAction(i, { kind: e.target.value as ActionKind, value: "" })}>
                {ACTION_KINDS.map((k) => (
                  <option key={k} value={k}>
                    {t(ACTION_NAMES[k])}
                  </option>
                ))}
              </select>
              {a.kind === "transition" ? (
                <select aria-label={t("Value of action {n}", { n: i + 1 })} value={a.value} onChange={(e) => setAction(i, { value: e.target.value })}>
                  <option value="">—</option>
                  {workflow.data?.statuses.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name}
                    </option>
                  ))}
                </select>
              ) : (
                <input aria-label={t("Value of action {n}", { n: i + 1 })} value={a.value} onChange={(e) => setAction(i, { value: e.target.value })} />
              )}
              {draft.actions.length > 1 && (
                <button type="button" className="ghost" aria-label={t("Remove action {n}", { n: i + 1 })}
                  onClick={() => setDraft((d) => ({ ...d, actions: d.actions.filter((_, j) => j !== i) }))}>
                  ✕
                </button>
              )}
            </div>
          ))}
          {draft.actions.length < 5 && (
            <button type="button" className="ghost" onClick={() => setDraft((d) => ({ ...d, actions: [...d.actions, { kind: "comment", value: "" }] }))}>
              {t("Add action")}
            </button>
          )}
        </fieldset>
        <button type="submit" disabled={!ruleReady(draft) || create.isPending}>
          {t("Create rule")}
        </button>
        {create.isError && (
          <p role="alert" className="error">
            {create.error.message}
          </p>
        )}
      </form>

      <h3>{t("Recent runs")}</h3>
      {runs.data?.length === 0 && <p className="muted">{t("No runs yet.")}</p>}
      <ul className="runs">
        {runs.data?.map((r, i) => (
          <li key={i} aria-label={`${r.issue_key} ${names.get(r.rule_id) ?? ""}`} className={r.ok ? "ok" : "failed"}>
            {r.ok ? "✓" : "✗"} <strong>{r.issue_key}</strong> · {names.get(r.rule_id) ?? t("Deleted rule")} · {r.detail}{" "}
            <time className="muted" dateTime={r.at}>
              {formatDateTime(r.at)}
            </time>
          </li>
        ))}
      </ul>
    </section>
  );
}
