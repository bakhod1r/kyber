import { useMutation, useQuery } from "@tanstack/react-query";
import { Link, useNavigate, useParams } from "react-router-dom";
import { formatDate, roleLabel } from "../../../shared/i18n/labels";
import { useT } from "../../../shared/i18n/i18n";
import { projectUseCases } from "../application";

/** InvitePage: opened from an invitation email (B1). Visitors sign in or sign up first and come back here. */
export function InvitePage({ signedIn }: { signedIn: boolean }) {
  const t = useT();
  const token = useParams().token ?? "";
  const navigate = useNavigate();
  const preview = useQuery({ queryKey: ["invitation", token], queryFn: () => projectUseCases.invitation(token), retry: false });
  const accept = useMutation({
    mutationFn: () => projectUseCases.acceptInvitation(token),
    onSuccess: (r) => navigate(`/projects/${encodeURIComponent(r.project)}`),
  });
  const here = encodeURIComponent(`/invite/${token}`);

  return (
    <main className="auth">
      <section className="panel invite-page">
        <img className="auth-logo" src="/logo.svg" alt="" width={48} height={48} />
        {preview.isPending && <p className="muted">{t("Loading…")}</p>}
        {preview.isError && (
          <p role="alert" className="error">
            {preview.error.message}
          </p>
        )}
        {preview.data && (
          <>
            <h1>{t("Join {project}", { project: preview.data.project_name })}</h1>
            <p>
              {t("{name} invited {email} to the project {project} as {role}.", {
                name: preview.data.invited_by,
                email: preview.data.email,
                project: `${preview.data.project_name} (${preview.data.project})`,
                role: roleLabel(t, preview.data.role),
              })}
            </p>
            {preview.data.expired ? (
              <p className="error">{t("This invitation has expired. Ask for a new one.")}</p>
            ) : signedIn ? (
              <>
                <p className="muted">{t("The link works until {date}.", { date: formatDate(preview.data.expires_at) })}</p>
                <button type="button" disabled={accept.isPending} onClick={() => accept.mutate()}>
                  {t("Accept invitation")}
                </button>
                {accept.isError && (
                  <p role="alert" className="error">
                    {accept.error.message}
                  </p>
                )}
              </>
            ) : (
              <p>
                {t("Sign in or sign up with {email} to accept.", { email: preview.data.email })}{" "}
                <Link to={`/signup?next=${here}`}>{t("Create an account")}</Link> · <Link to={`/login?next=${here}`}>{t("Log in")}</Link>
              </p>
            )}
          </>
        )}
      </section>
    </main>
  );
}
