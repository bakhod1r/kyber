// goal: application layer — the use cases the UI calls. They run through the gateway; the UI never
// imports infrastructure directly, so the transport can change without touching components.
import type { Goal, GoalStatus } from "../domain/model";
import { goalGateway } from "../infrastructure/gateway";

export const goalUseCases = {
  ...goalGateway,
  /** Reports how a goal is going, keeping its definition. */
  report: (g: Goal, status: GoalStatus) =>
    goalGateway.updateGoal(g.id, g.version, { name: g.name, target_date: g.target_date ?? "", jql: g.jql, status }),
};
export * from "../domain/model";
