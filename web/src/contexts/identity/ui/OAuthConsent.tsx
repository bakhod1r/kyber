import { useMutation, useQuery } from "@tanstack/react-query";
import { useSearchParams } from "react-router-dom";
import { authorizeRequest, identityUseCases, registeredRedirect } from "../application";
import { useT } from "../../../shared/i18n/i18n";

/** OAuthConsent: the signed-in user lets an MCP client (Claude, Cursor…) act as them (ADR-0016). */
export function OAuthConsent({ go = (url: string) => window.location.assign(url) }: { go?: (url: string) => void }) {
  const t = useT();
  const [params] = useSearchParams();
  const req = authorizeRequest(params);
  const client = useQuery({ queryKey: ["oauth-client", req?.client_id], queryFn: () => identityUseCases.oauthClient(req!.client_id), enabled: !!req });
  const approve = useMutation({ mutationFn: () => identityUseCases.approveOAuth(req!), onSuccess: (r) => go(r.redirect_to) });

  if (!req) return <p role="alert" className="error center">{t("This authorization request is incomplete.")}</p>;
  if (client.isPending) return <p className="muted center">{t("Loading…")}</p>;
  if (client.isError) return <p role="alert" className="error center">{client.error.message}</p>;
  if (!registeredRedirect(client.data, req.redirect_uri)) {
    return <p role="alert" className="error center">{t("This application is not allowed to use that redirect address.")}</p>;
  }
  const target = new URL(req.redirect_uri);
  function deny() {
    const to = new URL(req!.redirect_uri);
    to.searchParams.set("error", "access_denied");
    if (req!.state) to.searchParams.set("state", req!.state);
    go(to.toString());
  }
  return (
    <main className="auth">
      <section className="panel" aria-labelledby="oauth-title">
        <h1 id="oauth-title">{t("Allow {name} to access Kyber?", { name: client.data.client_name })}</h1>
        <p>{t("It will act as you: read and change the issues and projects you can access.")}</p>
        <p className="muted">
          {t("You will be sent back to")} <code>{target.host || target.protocol}</code>
        </p>
        <p className="muted">{t("You can revoke its access any time under Settings → Access tokens.")}</p>
        {approve.isError && <p role="alert" className="error">{approve.error.message}</p>}
        <div className="row">
          <button type="button" onClick={() => approve.mutate()} disabled={approve.isPending}>
            {t("Allow")}
          </button>
          <button type="button" className="ghost" onClick={deny}>
            {t("Deny")}
          </button>
        </div>
      </section>
    </main>
  );
}
