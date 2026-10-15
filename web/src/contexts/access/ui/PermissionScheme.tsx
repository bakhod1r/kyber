import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";
import { accessUseCases, type GrantKind, type PermissionGrants } from "../application";
import { ApiError } from "../../../shared/http";
import { projectUseCases } from "../../project/application";
import { msg, type MessageKey, useLabel, useT } from "../../../shared/i18n/i18n";

/** Holders shown as columns; individual users are listed per permission. */
const COLUMNS: { kind: GrantKind; value: string; label: MessageKey }[] = [
  { kind: "role", value: "admin", label: msg("Admin") },
  { kind: "role", value: "member", label: msg("Member") },
  { kind: "role", value: "viewer", label: msg("Viewer") },
  { kind: "reporter", value: "", label: msg("Reporter") },
  { kind: "assignee", value: "", label: msg("Assignee") },
];

/** Admins must keep these, so the project cannot lock its admins out (the API enforces it too). */
const LOCKED = new Set(["BROWSE_PROJECTS", "ADMINISTER_PROJECTS"]);

const format = (p: string) => p.charAt(0) + p.slice(1).toLowerCase().replaceAll("_", " ");

/** PermissionScheme: the project's permission scheme editor (project admins). */
export function PermissionScheme({ projectKey }: { projectKey: string }) {
  const label = useLabel();
  const t = useT();
  const qc = useQueryClient();
  const key = ["permissions", projectKey];
  const scheme = useQuery({ queryKey: key, queryFn: () => accessUseCases.permissions(projectKey), retry: false });
  const members = useQuery({ queryKey: ["members", projectKey], queryFn: () => projectUseCases.members(projectKey) });
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
  const save = useMutation({ mutationFn: (p: PermissionGrants[]) => accessUseCases.setPermissions(projectKey, p), ...done(t("Saved")) });
  const reset = useMutation({ mutationFn: () => accessUseCases.resetPermissions(projectKey), ...done(t("Reset to the default scheme")) });

  if (scheme.isPending || !draft) {
    if (scheme.isError) {
      const forbidden = scheme.error instanceof ApiError && scheme.error.status === 403;
      return <p className="muted">{forbidden ? t("Only project admins can change permissions.") : scheme.error.message}</p>;
    }
    return <p className="muted">{t("Loading…")}</p>;
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
    <section className="permissions panel" aria-label={t("Permissions")}>
      <h2>{t("Permissions")}</h2>
      <p className="muted">
        {scheme.data?.custom ? t("This project has its own permission scheme.") : t("This project uses the default permission scheme.")}{" "}
        {t("Reporter and Assignee apply to the issue at hand.")}
      </p>
      <table className="perm-table">
        <thead>
          <tr>
            <th scope="col">{t("Permission")}</th>
            {COLUMNS.map((c) => (
              <th scope="col" key={c.label}>
                {t(c.label)}
              </th>
            ))}
            <th scope="col">{t("Users")}</th>
          </tr>
        </thead>
        <tbody>
          {draft.map((p) => (
            <tr key={p.permission}>
              <th scope="row">{label(format(p.permission))}</th>
              {COLUMNS.map((c) => {
                const locked = LOCKED.has(p.permission) && c.value === "admin";
                return (
                  <td key={c.label}>
                    <input
                      type="checkbox"
                      aria-label={`${label(format(p.permission))}: ${t(c.label)}`}
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
                      aria-label={t("Remove {name} from {permission}", { name: name(g.value), permission: label(format(p.permission)) })}
                      onClick={() => toggle(p.permission, "user", g.value)}
                    >
                      {name(g.value)} ×
                    </button>
                  ))}
                <select
                  aria-label={t("Add a user to {permission}", { permission: label(format(p.permission)) })}
                  value=""
                  onChange={(e) => e.target.value && toggle(p.permission, "user", e.target.value)}
                >
                  <option value="">{t("+ user")}</option>
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
            {t("Reset to default")}
          </button>
        )}
        <button type="button" onClick={() => save.mutate(draft)} disabled={save.isPending}>
          {t("Save permissions")}
        </button>
      </div>
    </section>
  );
}
