// agile: HTTP gateway to the agile API (infrastructure).
import { request, requestText, seg, type List } from "../../../shared/http";
import type { BoardView, Completion, Sprint, Version, VersionInput } from "../domain/model";

export const agileGateway = {
  sprints: (key: string) => request<List<Sprint>>("GET", `/api/v1/projects/${seg(key)}/sprints`).then((r) => r.items),
  createSprint: (key: string, name: string, goal: string) =>
    request<Sprint>("POST", `/api/v1/projects/${seg(key)}/sprints`, { name, goal }),
  startSprint: (id: string, endsAt?: string) =>
    request<Sprint>("POST", `/api/v1/sprints/${seg(id)}/start`, endsAt ? { ends_at: endsAt } : undefined),
  completeSprint: (id: string, moveTo = "") =>
    request<Completion>("POST", `/api/v1/sprints/${seg(id)}/complete`, moveTo ? { move_to: moveTo } : undefined),
  versions: (key: string) => request<List<Version>>("GET", `/api/v1/projects/${seg(key)}/versions`).then((r) => r.items),
  createVersion: (key: string, v: VersionInput) => request<Version>("POST", `/api/v1/projects/${seg(key)}/versions`, v),
  editVersion: (id: string, version: number, v: VersionInput) => request<Version>("PATCH", `/api/v1/versions/${seg(id)}`, { version, ...v }),
  releaseVersion: (id: string, moveTo = "") =>
    request<{ version: Version; moved: number }>("POST", `/api/v1/versions/${seg(id)}/release`, moveTo ? { move_to: moveTo } : undefined),
  versionStep: (id: string, step: "unrelease" | "archive" | "restore") => request<Version>("POST", `/api/v1/versions/${seg(id)}/${step}`),
  deleteVersion: (id: string, moveTo = "") =>
    request<undefined>("DELETE", `/api/v1/versions/${seg(id)}${moveTo ? `?move_to=${seg(moveTo)}` : ""}`),
  releaseNotes: (id: string) => requestText(`/api/v1/versions/${seg(id)}/release-notes`),
  updateBoard: (b: BoardView, patch: Partial<Pick<BoardView, "name" | "kind" | "columns" | "kanban_backlog">>) =>
    request<BoardView>("PUT", `/api/v1/boards/${seg(b.id)}`, {
      version: b.version, name: b.name, kind: b.kind, columns: b.columns, kanban_backlog: b.kanban_backlog ?? false, ...patch,
    }),
  boards: (key: string) => request<List<BoardView>>("GET", `/api/v1/projects/${seg(key)}/boards`).then((r) => r.items),
};
