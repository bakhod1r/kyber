// automation: domain model (types and pure rules) — no I/O.
export type Trigger = "issue.created" | "issue.transitioned" | "comment.added" | "issue.assigned";
export const TRIGGERS: Trigger[] = ["issue.created", "issue.transitioned", "comment.added", "issue.assigned"];
export type ActionKind = "transition" | "assign" | "add_label" | "comment";
export const ACTION_KINDS: ActionKind[] = ["transition", "assign", "add_label", "comment"];
export type Action = { kind: ActionKind; value: string };
export type RuleInput = { name: string; trigger: Trigger; to_status: string; condition: string; actions: Action[] };
export type Rule = RuleInput & { id: string; project_key: string; owner_id: string; enabled: boolean; version: number };
export type Run = { rule_id: string; issue_key: string; at: string; ok: boolean; detail: string };

/** A rule's form is ready to save: a name, and every action has a value. */
export function ruleReady(r: RuleInput): boolean {
  return r.name.trim() !== "" && r.actions.length > 0 && r.actions.every((a) => a.value.trim() !== "");
}
