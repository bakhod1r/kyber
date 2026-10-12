import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { ApiError, type GrantKind, type PermissionGrants, api } from "../api";

/** Holders shown as columns; individual users are listed per permission. */
const COLUMNS: { kind: GrantKind; value: string; label: string }[] = [
  { kind: "role", value: "admin", label: "Admin" },
  { kind: "role", value: "member", label: "Member" },
  { kind: "role", value: "viewer", label: "Viewer" },
  { kind: "reporter", value: "", label: "Reporter" },
  { kind: "assignee", value: "", label: "Assignee" },
];

/** Admins must keep these, so the project cannot lock its admins out (the API enforces it too). */
const LOCKED = new Set(["BROWSE_PROJECTS", "ADMINISTER_PROJECTS"]);

const label = (p: string) => p.charAt(0) + p.slice(1).toLowerCase().replaceAll("_", " ");

/** PermissionScheme: the project's permission scheme editor (project admins). */
export function PermissionScheme({ projectKey }: { projectKey: string }) {
  const qc = useQueryClient();
  const key = ["permissions", projectKey];
  const scheme = useQuery({ queryKey: key, queryFn: () => api.permissions(projectKey), retry: false });
  const members = useQuery({ queryKey: ["members", projectKey], queryFn: () => api.members(projectKey) });
  const [draft, setDraft] = useState<PermissionGrants[] | null>(null);
  const [notice, setNotice] = useState<{ kind: "ok" | "error"; text: string } | null>(null);
  useEffect(() => {
    if (scheme.data) setDraft(scheme.data.permissions);
  }, [scheme.data]);

  const done = (text: string) => ({
    onSuccess: () => {
      setNotice({ kind: "ok" as const, text });
      return qc.invalidateQueries({ queryKey: key });
    },
    onError: (err: Error) => setNotice({ kind: "error" as const, text: err.message }),
  });
  const save = useMutation({ mutationFn: (p: PermissionGrants[]) => api.setPermissions(projectKey, p), ...done("Saved") });
  const reset = useMutation({ mutationFn: () => api.resetPermissions(projectKey), ...done("Reset to the default scheme") });

  if (scheme.isPending || !draft) {
    if (scheme.isError) {
      const forbidden = scheme.error instanceof ApiError && scheme.error.status === 403;
      return <p className="muted">{forbidden ? "Only project admins can change permissions." : scheme.error.message}</p>;
    }
    return <p className="muted">Loading…</p>;
  }

  const has = (p: PermissionGrants, kind: GrantKind, value: string) => p.grants.some((g) => g.kind === kind && g.value === value);
  const update = (perm: string, change: (p: PermissionGrants) => PermissionGrants) =>
    setDraft(draft.map((p) => (p.permission === perm ? change(p) : p)));
  const toggle = (perm: string, kind: GrantKind, value: string) =>
    update(perm, (p) => ({
      ...p,
      grants: has(p, kind, value) ? p.grants.filter((g) => !(g.kind === kind && g.value === value)) : [...p.grants, { kind, value }],
    }));
  const name = (id: string) => members.data?.find((m) => m.user_id === id)?.name ?? id;

  return (
    <section className="permissions panel" aria-label="Permissions">
      <h2>Permissions</h2>
      <p className="muted">
        {scheme.data?.custom ? "This project has its own permission scheme." : "This project uses the default permission scheme."} Reporter and
        Assignee apply to the issue at hand.
      </p>
      <table className="perm-table">
        <thead>
          <tr>
            <th scope="col">Permission</th>
            {COLUMNS.map((c) => (
              <th scope="col" key={c.label}>
                {c.label}
              </th>
            ))}
            <th scope="col">Users</th>
          </tr>
        </thead>
        <tbody>
          {draft.map((p) => (
            <tr key={p.permission}>
              <th scope="row">{label(p.permission)}</th>
              {COLUMNS.map((c) => {
                const locked = LOCKED.has(p.permission) && c.value === "admin";
                return (
                  <td key={c.label}>
                    <input
                      type="checkbox"
                      aria-label={`${label(p.permission)}: ${c.label}`}
                      checked={has(p, c.kind, c.value)}
                      disabled={locked}
                      onChange={() => toggle(p.permission, c.kind, c.value)}
                    />
                  </td>
                );
              })}
              <td>
                {p.grants
                  .filter((g) => g.kind === "user")
                  .map((g) => (
                    <button
                      key={g.value}
                      type="button"
                      className="ghost chip"
                      aria-label={`Remove ${name(g.value)} from ${label(p.permission)}`}
                      onClick={() => toggle(p.permission, "user", g.value)}
                    >
                      {name(g.value)} ×
                    </button>
                  ))}
                <select
                  aria-label={`Add a user to ${label(p.permission)}`}
                  value=""
                  onChange={(e) => e.target.value && toggle(p.permission, "user", e.target.value)}
                >
                  <option value="">+ user</option>
                  {members.data
                    ?.filter((m) => !has(p, "user", m.user_id))
                    .map((m) => (
                      <option key={m.user_id} value={m.user_id}>
                        {m.name}
                      </option>
                    ))}
                </select>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      <div className="row end">
        {notice && (
          <span role={notice.kind === "error" ? "alert" : "status"} className={notice.kind === "error" ? "error" : "muted"}>
            {notice.text}
          </span>
        )}
        {scheme.data?.custom && (
          <button type="button" className="ghost" onClick={() => reset.mutate()} disabled={reset.isPending}>
            Reset to default
          </button>
        )}
        <button type="button" onClick={() => save.mutate(draft)} disabled={save.isPending}>
          Save permissions
        </button>
      </div>
    </section>
  );
}
