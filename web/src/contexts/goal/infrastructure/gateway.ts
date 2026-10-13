// goal: HTTP gateway to the goals API (infrastructure).
import { request, seg, type List } from "../../../shared/http";
import type { Goal, GoalInput, GoalStatus } from "../domain/model";

export const goalGateway = {
  goals: (key: string) => request<List<Goal>>("GET", `/api/v1/projects/${seg(key)}/goals`).then((r) => r.items),
  createGoal: (key: string, g: GoalInput) => request<Goal>("POST", `/api/v1/projects/${seg(key)}/goals`, g),
  updateGoal: (id: string, version: number, g: GoalInput & { status: GoalStatus }) =>
    request<Goal>("PUT", `/api/v1/goals/${seg(id)}`, { version, ...g }),
  deleteGoal: (id: string) => request<undefined>("DELETE", `/api/v1/goals/${seg(id)}`),
};
