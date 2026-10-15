import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { adminUseCases, type ConnectionScope, type NewSSOConnection } from "../application";
import { useT } from "../../../shared/i18n/i18n";
import { ConnectionDomains } from "./ConnectionDomains";

const empty: NewSSOConnection = { name: "", issuer: "", client_id: "", client_secret: "", groups_claim: "", scope: "workspace" };

/** SSOConnections: site administrators add identity providers people can sign in with (ADR-0018). Hidden from everyone else. */
export function SSOConnections() {
  const t = useT();
  const qc = useQueryClient();
  const list = useQuery({ queryKey: ["admin", "sso-connections"], queryFn: adminUseCases.connections, retry: false });
  const [form, setForm] = useState<NewSSOConnection>(empty);
  const [error, setError] = useState("");
  const refresh = () => {
    setError("");
    return qc.invalidateQueries({ queryKey: ["admin", "sso-connections"] });
  };
  const fail = (e: Error) => setError(e.message);
  const add = useMutation({
    mutationFn: adminUseCases.addConnection,
    onSuccess: () => {
      setForm(empty);
      return refresh();
    },
    onError: fail,
  });
  const toggle = useMutation({ mutationFn: ({ id, on }: { id: string; on: boolean }) => adminUseCases.setConnectionEnabled(id, on), onSuccess: refresh, onError: fail });
  const enforce = useMutation({ mutationFn: ({ id, on }: { id: string; on: boolean }) => adminUseCases.setConnectionEnforced(id, on), onSuccess: refresh, onError: fail });
  const remove = useMutation({ mutationFn: adminUseCases.removeConnection, onSuccess: refresh, onError: fail });

  if (!list.data) return null;
  const submit = (e: FormEvent) => {
    e.preventDefault();
    add.mutate(form);
  };
  const field = (key: keyof NewSSOConnection, label: string, type = "text", required = true) => (
    <input aria-label={label} placeholder={label} type={type} value={form[key]} required={required} autoComplete="off" onChange={(e) => setForm({ ...form, [key]: e.target.value })} />
  );
  return (
    <section aria-label={t("SSO connections")} className="panel">
      <h2>{t("SSO connections")}</h2>
      <p className="muted">{t("OpenID Connect providers shown on the login page. Register each one's callback URL at the provider.")}</p>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {list.data.length === 0 && <p className="muted">{t("No SSO connections yet.")}</p>}
      <ul>
        {list.data.map((c) => (
          <li key={c.id}>
            <b>{c.name}</b> · <span className="muted">{c.issuer}</span> {!c.enabled && <span className="muted">({t("disabled")})</span>}
            <div>
              {c.workspace_id ? (
                <label>
                  <input
                    type="checkbox"
                    aria-label={t("Require {name} for this workspace", { name: c.name })}
                    checked={c.enforced ?? false}
                    onChange={(e) => enforce.mutate({ id: c.id, on: e.target.checked })}
                  />{" "}
                  {t("Required: members must sign in with it (owners exempt)")}
                </label>
              ) : (
                <span className="muted">{t("installation-wide")}</span>
              )}
            </div>
            <div>
              <code>{`${location.origin}/api/v1/auth/sso/${c.id}/callback`}</code>
            </div>
            <button type="button" className="ghost" onClick={() => toggle.mutate({ id: c.id, on: !c.enabled })}>
              {c.enabled ? t("Disable {name}", { name: c.name }) : t("Enable {name}", { name: c.name })}
            </button>
            <button type="button" className="ghost" aria-label={t("Remove connection {name}", { name: c.name })} onClick={() => remove.mutate(c.id)}>
              ×
            </button>
            <ConnectionDomains connection={c} onError={setError} />
          </li>
        ))}
      </ul>
      <form onSubmit={submit}>
        {field("name", t("Button label"))}
        {field("issuer", t("Issuer URL"), "url")}
        {field("client_id", t("Client ID"))}
        {field("client_secret", t("Client secret"), "password")}
        {field("groups_claim", t("Groups claim (optional)"), "text", false)}
        <select aria-label={t("Scope")} value={form.scope} onChange={(e) => setForm({ ...form, scope: e.target.value as ConnectionScope })}>
          <option value="workspace">{t("This workspace")}</option>
          <option value="installation">{t("Whole installation (site admins)")}</option>
        </select>
        <button type="submit">{t("Add connection")}</button>
      </form>
    </section>
  );
}
