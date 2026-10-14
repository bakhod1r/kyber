import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { projectUseCases, type Role } from "../application";
import { useT } from "../../../shared/i18n/i18n";

/** Members: the project's people and their roles (project admins change them). */
export function Members({ projectKey }: { projectKey: string }) {
  const t = useT();
  const qc = useQueryClient();
  const members = useQuery({ queryKey: ["members", projectKey], queryFn: () => projectUseCases.members(projectKey) });
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<Role>("member");
  const invite = useMutation({
    mutationFn: () => projectUseCases.setMember(projectKey, email, role),
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
      <h2>{t("Members")}</h2>
      <ul>
        {members.data?.map((m) => (
          <li key={m.user_id}>
            <span>{m.name}</span> <span className={`role role-${m.role}`}>{m.role}</span>
          </li>
        ))}
      </ul>
      <form onSubmit={onSubmit}>
        <label>
          {t("Invite by email")}
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
        </label>
        <label>
          {t("Role")}
          <select value={role} onChange={(e) => setRole(e.target.value as Role)}>
            <option value="member">{t("member")}</option>
            <option value="viewer">{t("viewer")}</option>
            <option value="admin">{t("admin")}</option>
          </select>
        </label>
        <button type="submit" disabled={invite.isPending}>
          {t("Invite")}
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
