import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { useParams } from "react-router-dom";
import { type Role, api } from "../api";
import { Board } from "../components/Board";
import { IssuePanel } from "../components/IssuePanel";

export function ProjectPage() {
  const key = useParams<{ key: string }>().key ?? "";
  const projects = useQuery({ queryKey: ["projects"], queryFn: api.projects });
  const project = projects.data?.find((p) => p.key === key);
  const [open, setOpen] = useState<string | null>(null);
  return (
    <div className="project">
      <h1>
        <span className="key">{key}</span> {project?.name ?? ""}
      </h1>
      <div className="project-grid">
        <Board projectKey={key} onOpen={setOpen} />
        <Members projectKey={key} />
      </div>
      {open && <IssuePanel issueKey={open} projectKey={key} onClose={() => setOpen(null)} />}
    </div>
  );
}

function Members({ projectKey }: { projectKey: string }) {
  const qc = useQueryClient();
  const members = useQuery({ queryKey: ["members", projectKey], queryFn: () => api.members(projectKey) });
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<Role>("member");
  const invite = useMutation({
    mutationFn: () => api.setMember(projectKey, email, role),
    onSuccess: () => {
      setEmail("");
      return qc.invalidateQueries({ queryKey: ["members", projectKey] });
    },
  });

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    invite.mutate();
  }

  return (
    <aside className="panel members">
      <h2>Members</h2>
      <ul>
        {members.data?.map((m) => (
          <li key={m.user_id}>
            <span>{m.name}</span> <span className={`role role-${m.role}`}>{m.role}</span>
          </li>
        ))}
      </ul>
      <form onSubmit={onSubmit}>
        <label>
          Invite by email
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
        </label>
        <label>
          Role
          <select value={role} onChange={(e) => setRole(e.target.value as Role)}>
            <option value="member">member</option>
            <option value="viewer">viewer</option>
            <option value="admin">admin</option>
          </select>
        </label>
        <button type="submit" disabled={invite.isPending}>
          Invite
        </button>
        {invite.isError && (
          <p role="alert" className="error">
            {invite.error.message}
          </p>
        )}
      </form>
    </aside>
  );
}
