import { useMutation } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { identityUseCases } from "../application";
import { useT } from "../../../shared/i18n/i18n";

/** SSODiscovery: the work email finds its company's identity provider (ADR-0018, S107). */
export function SSODiscovery() {
  const t = useT();
  const [email, setEmail] = useState("");
  const find = useMutation({ mutationFn: identityUseCases.discoverSSO });
  const submit = (e: FormEvent) => {
    e.preventDefault();
    find.mutate(email.trim());
  };
  const found = find.data;
  return (
    <form aria-label={t("Single sign-on")} className="sso-discovery" onSubmit={submit}>
      <input type="email" aria-label={t("Work email")} placeholder={t("Work email")} value={email} required onChange={(e) => setEmail(e.target.value)} />
      <button type="submit" className="ghost">
        {t("Find my sign-in")}
      </button>
      {found && (
        <a className="button social-sso" href={`/api/v1/auth/sso/${encodeURIComponent(found.id)}/start`}>
          {t("Continue with {name}", { name: found.name })}
        </a>
      )}
      {found === null && <p role="status">{t("No single sign-on for this email. Use another way to sign in.")}</p>}
      {find.isError && (
        <p role="alert" className="error">
          {find.error.message}
        </p>
      )}
    </form>
  );
}
