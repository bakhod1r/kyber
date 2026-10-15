import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { adminUseCases, MAPPED_ROLES, type MappingScope } from "../application";
import { useT } from "../../../shared/i18n/i18n";

/** GroupMappings: site administrators map identity-provider groups to roles (S106). Hidden from everyone else. */
export function GroupMappings() {
  const t = useT();
  const qc = useQueryClient();
  const mappings = useQuery({ queryKey: ["admin", "sso-groups"], queryFn: adminUseCases.groupMappings, retry: false });
  const [group, setGroup] = useState("");
  const [scope, setScope] = useState<MappingScope>("workspace");
  const [projectKey, setProjectKey] = useState("");
  const [role, setRole] = useState("member");
  const [error, setError] = useState("");
  const refresh = () => {
    setError("");
    return qc.invalidateQueries({ queryKey: ["admin", "sso-groups"] });
  };
  const add = useMutation({
    mutationFn: adminUseCases.addGroupMapping,
    onSuccess: () => {
      setGroup("");
      setProjectKey("");
      return refresh();
    },
    onError: (e: Error) => setError(e.message),
  });
  const remove = useMutation({ mutationFn: adminUseCases.removeGroupMapping, onSuccess: refresh, onError: (e: Error) => setError(e.message) });

  if (!mappings.data) return null;
  const submit = (e: FormEvent) => {
    e.preventDefault();
    add.mutate(scope === "project" ? { group, scope, project_key: projectKey, role } : { group, scope, role });
  };
  const changeScope = (s: MappingScope) => {
    setScope(s);
    if (!MAPPED_ROLES[s].includes(role)) setRole("member");
  };
  return (
    <section aria-label={t("SSO group mappings")} className="panel">
      <h2>{t("SSO group mappings")}</h2>
      <p className="muted">{t("Members of an identity-provider group get the mapped role at every sign-in.")}</p>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {mappings.data.length === 0 && <p className="muted">{t("No group mappings yet.")}</p>}
      <ul>
        {mappings.data.map((m) => (
          <li key={m.id}>
            <b>{m.group}</b> → {m.scope === "project" ? <span className="key">{m.project_key}</span> : t("Workspace")} · {m.role}{" "}
            <button type="button" className="ghost" aria-label={t("Remove mapping {group}", { group: m.group })} onClick={() => remove.mutate(m.id)}>
              ×
            </button>
          </li>
        ))}
      </ul>
      <form onSubmit={submit}>
        <input aria-label={t("Group")} value={group} required onChange={(e) => setGroup(e.target.value)} />
        <select aria-label={t("Scope")} value={scope} onChange={(e) => changeScope(e.target.value as MappingScope)}>
          <option value="workspace">{t("Workspace")}</option>
          <option value="project">{t("Project")}</option>
        </select>
        {scope === "project" && <input aria-label={t("Project key")} value={projectKey} required onChange={(e) => setProjectKey(e.target.value)} />}
        <select aria-label={t("Mapped role")} value={role} onChange={(e) => setRole(e.target.value)}>
          {MAPPED_ROLES[scope].map((r) => (
            <option key={r} value={r}>
              {r}
            </option>
          ))}
        </select>
        <button type="submit">{t("Add mapping")}</button>
      </form>
    </section>
  );
}
