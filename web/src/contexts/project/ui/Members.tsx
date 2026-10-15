import { formatDate, roleLabel } from "../../../shared/i18n/labels";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { projectUseCases, type InviteResult, type Role } from "../application";
import { useT } from "../../../shared/i18n/i18n";

/** Members: the project's people and their roles; admins invite by email and manage pending invitations (B1). */
export function Members({ projectKey }: { projectKey: string }) {
  const t = useT();
  const qc = useQueryClient();
  const members = useQuery({ queryKey: ["members", projectKey], queryFn: () => projectUseCases.members(projectKey) });
  // Only admins may list invitations; for everyone else the query fails quietly and the list stays hidden.
  const invitations = useQuery({ queryKey: ["invitations", projectKey], queryFn: () => projectUseCases.invitations(projectKey), retry: false });
  const [email, setEmail] = useState("");
  const [role, setRole] = useState<Role>("member");
  const [result, setResult] = useState<{ email: string; res: InviteResult } | null>(null);
  const invite = useMutation({
    mutationFn: () => projectUseCases.invite(projectKey, email, role),
    onSuccess: (res) => {
      setResult({ email, res });
      setEmail("");
      return Promise.all([
        qc.invalidateQueries({ queryKey: ["members", projectKey] }),
        qc.invalidateQueries({ queryKey: ["invitations", projectKey] }),
      ]);
    },
  });
  const revoke = useMutation({
    mutationFn: (id: string) => projectUseCases.revokeInvitation(projectKey, id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["invitations", projectKey] }),
  });

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    setResult(null);
    invite.mutate();
  }

  return (
    <aside className="panel members">
      <h2>{t("Members")}</h2>
      <ul>
        {members.data?.map((m) => (
          <li key={m.user_id}>
            <span>{m.name}</span> <span className={`role role-${m.role}`}>{roleLabel(t, m.role)}</span>
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
            <option value="member">{roleLabel(t, "member")}</option>
            <option value="viewer">{roleLabel(t, "viewer")}</option>
            <option value="admin">{roleLabel(t, "admin")}</option>
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
        {result && (
          <div role="status" className="invite-result">
            {result.res.added ? (
              t("{email} was added to the project.", { email: result.email })
            ) : result.res.emailed ? (
              t("Invitation emailed to {email}.", { email: result.email })
            ) : (
              <>
                {t("Email is not configured; share this link with {email}:", { email: result.email })}
                <input readOnly aria-label={t("Invitation link")} value={result.res.link} onFocus={(e) => e.currentTarget.select()} />
              </>
            )}
          </div>
        )}
      </form>
      {invitations.isSuccess && (
        <section className="invitations">
          <h3>{t("Pending invitations")}</h3>
          {invitations.data.length === 0 ? (
            <p className="muted">{t("No pending invitations.")}</p>
          ) : (
            <ul aria-label={t("Pending invitations")}>
              {invitations.data.map((i) => (
                <li key={i.id}>
                  <span>
                    {i.email}
                    <small className="muted">
                      {" "}
                      {roleLabel(t, i.role)} · {t("until {date}", { date: formatDate(i.expires_at) })}
                    </small>
                  </span>
                  <button
                    type="button"
                    className="ghost"
                    aria-label={t("Revoke invitation for {email}", { email: i.email })}
                    disabled={revoke.isPending}
                    onClick={() => revoke.mutate(i.id)}
                  >
                    ✕
                  </button>
                </li>
              ))}
            </ul>
          )}
        </section>
      )}
    </aside>
  );
}
