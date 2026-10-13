import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { adminUseCases, ROLES, type Role } from "../application";

/** AdminPage: a workspace's administrators manage roles and look after projects and stuck events. */
export function AdminPage() {
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
      setNotice("Role changed");
      return qc.invalidateQueries({ queryKey: ["admin", "members"] });
    },
    onError: fail,
  });
  const retry = useMutation({
    mutationFn: adminUseCases.retry,
    onSuccess: () => {
      setError("");
      setNotice("Queued again");
      return qc.invalidateQueries({ queryKey: ["admin", "dead-letters"] });
    },
    onError: fail,
  });

  const refusal = access.data === false ? "Only workspace administrators can see this page." : (access.error ?? members.error)?.message;
  if (refusal) {
    return (
      <div className="admin">
        <h1>Administration</h1>
        <p role="alert" className="error">
          {refusal}
        </p>
      </div>
    );
  }
  return (
    <div className="admin">
      <h1>Administration</h1>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {notice && <p role="status">{notice}</p>}

      <section aria-label="Members" className="panel">
        <h2>Members</h2>
        <table>
          <thead>
            <tr>
              <th>Name</th>
              <th>Email</th>
              <th>Role</th>
            </tr>
          </thead>
          <tbody>
            {members.data?.map((m) => (
              <tr key={m.id}>
                <td>{m.name}</td>
                <td>{m.email}</td>
                <td>
                  <select aria-label={`Role of ${m.name}`} value={m.role} onChange={(e) => changeRole.mutate({ id: m.id, role: e.target.value as Role })}>
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

      <section aria-label="Projects" className="panel">
        <h2>Projects</h2>
        {projects.data?.length === 0 && <p className="muted">No projects yet.</p>}
        <ul>
          {projects.data?.map((p) => (
            <li key={p.key}>
              <span className="key">{p.key}</span> {p.name} · <span className="muted">{p.members} {p.members === 1 ? "member" : "members"}</span>
            </li>
          ))}
        </ul>
      </section>

      <section aria-label="Stuck events" className="panel">
        <h2>Stuck events</h2>
        <p className="muted">Events whose handlers failed 10 times. Fix the cause, then queue them again.</p>
        {dead.data?.length === 0 && <p className="muted">Nothing is stuck.</p>}
        <ul>
          {dead.data?.map((d) => (
            <li key={d.id}>
              <code>#{d.id}</code> <b>{d.event}</b> · <span className="muted">{d.attempts} attempts · {new Date(d.at).toLocaleString()}</span>
              <div className="error">{d.last_error}</div>
              <button type="button" className="ghost" onClick={() => retry.mutate(d.id)}>
                Retry #{d.id}
              </button>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
