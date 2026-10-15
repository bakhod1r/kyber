import { formatDateTime } from "../../../shared/i18n/labels";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { adminUseCases, ROLES, type Role } from "../application";
import { useT } from "../../../shared/i18n/i18n";
import { GroupMappings } from "./GroupMappings";
import { SSOConnections } from "./SSOConnections";

/** AdminPage: a workspace's administrators manage roles and look after projects and stuck events. */
export function AdminPage() {
  const t = useT();
  const qc = useQueryClient();
  const access = useQuery({ queryKey: ["admin", "access"], queryFn: adminUseCases.access, retry: false });
  const enabled = access.data === true;
  const members = useQuery({ queryKey: ["admin", "members"], queryFn: adminUseCases.members, enabled });
  const projects = useQuery({ queryKey: ["admin", "projects"], queryFn: adminUseCases.projects, enabled });
  const dead = useQuery({ queryKey: ["admin", "dead-letters"], queryFn: adminUseCases.deadLetters, enabled });
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const fail = (e: Error) => {
    setNotice("");
    setError(e.message);
  };
  const changeRole = useMutation({
    mutationFn: ({ id, role }: { id: string; role: Role }) => adminUseCases.changeRole(id, role),
    onSuccess: () => {
      setError("");
      setNotice(t("Role changed"));
      return qc.invalidateQueries({ queryKey: ["admin", "members"] });
    },
    onError: fail,
  });
  const retry = useMutation({
    mutationFn: adminUseCases.retry,
    onSuccess: () => {
      setError("");
      setNotice(t("Queued again"));
      return qc.invalidateQueries({ queryKey: ["admin", "dead-letters"] });
    },
    onError: fail,
  });

  const refusal = access.data === false ? t("Only workspace administrators can see this page.") : (access.error ?? members.error)?.message;
  if (refusal) {
    return (
      <div className="admin">
        <h1>{t("Administration")}</h1>
        <p role="alert" className="error">
          {refusal}
        </p>
      </div>
    );
  }
  return (
    <div className="admin">
      <h1>{t("Administration")}</h1>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {notice && <p role="status">{notice}</p>}

      <section aria-label={t("Members")} className="panel">
        <h2>{t("Members")}</h2>
        <table>
          <thead>
            <tr>
              <th>{t("Name")}</th>
              <th>{t("Email")}</th>
              <th>{t("Role")}</th>
            </tr>
          </thead>
          <tbody>
            {members.data?.map((m) => (
              <tr key={m.id}>
                <td>{m.name}</td>
                <td>{m.email}</td>
                <td>
                  <select aria-label={t("Role of {name}", { name: m.name })} value={m.role} onChange={(e) => changeRole.mutate({ id: m.id, role: e.target.value as Role })}>
                    {ROLES.map((r) => (
                      <option key={r} value={r}>
                        {r}
                      </option>
                    ))}
                  </select>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>

      <section aria-label={t("Projects")} className="panel">
        <h2>{t("Projects")}</h2>
        {projects.data?.length === 0 && <p className="muted">{t("No projects yet.")}</p>}
        <ul>
          {projects.data?.map((p) => (
            <li key={p.key}>
              <span className="key">{p.key}</span> {p.name} · <span className="muted">{(p.members === 1 ? t("{n} member", { n: p.members }) : t("{n} members", { n: p.members }))}</span>
            </li>
          ))}
        </ul>
      </section>

      <section aria-label={t("Stuck events")} className="panel">
        <h2>{t("Stuck events")}</h2>
        <p className="muted">{t("Events whose handlers failed 10 times. Fix the cause, then queue them again.")}</p>
        {dead.data?.length === 0 && <p className="muted">{t("Nothing is stuck.")}</p>}
        <ul>
          {dead.data?.map((d) => (
            <li key={d.id}>
              <code>#{d.id}</code> <b>{d.event}</b> · <span className="muted">{t("{n} attempts", { n: d.attempts })} · {formatDateTime(d.at)}</span>
              <div className="error">{d.last_error}</div>
              <button type="button" className="ghost" onClick={() => retry.mutate(d.id)}>
                {t("Retry #{id}", { id: d.id })}
              </button>
            </li>
          ))}
        </ul>
      </section>

      <SSOConnections />
      <GroupMappings />
    </div>
  );
}
