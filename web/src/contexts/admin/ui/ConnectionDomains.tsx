import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { adminUseCases, type SSOConnection } from "../application";
import { useT } from "../../../shared/i18n/i18n";

/** ConnectionDomains: the email domains a connection serves; each routes sign-in once its TXT record is verified (ADR-0018 §4). */
export function ConnectionDomains({ connection: c, onError }: { connection: SSOConnection; onError: (message: string) => void }) {
  const t = useT();
  const qc = useQueryClient();
  const [domain, setDomain] = useState("");
  const [trusted, setTrusted] = useState(false);
  const done = () => {
    onError("");
    return qc.invalidateQueries({ queryKey: ["admin", "sso-connections"] });
  };
  const fail = (e: Error) => onError(e.message);
  const add = useMutation({
    mutationFn: () => adminUseCases.addDomain(c.id, domain.trim(), trusted),
    onSuccess: () => {
      setDomain("");
      setTrusted(false);
      return done();
    },
    onError: fail,
  });
  const verify = useMutation({ mutationFn: (name: string) => adminUseCases.verifyDomain(c.id, name), onSuccess: done, onError: fail });
  const remove = useMutation({ mutationFn: (name: string) => adminUseCases.removeDomain(c.id, name), onSuccess: done, onError: fail });
  const submit = (e: FormEvent) => {
    e.preventDefault();
    add.mutate();
  };
  return (
    <div className="sso-domains">
      <ul>
        {c.domains.map((d) => (
          <li key={d.name}>
            <span>{d.name}</span>{" "}
            {d.verified ? (
              <span className="muted">✓ {t("verified")}</span>
            ) : (
              <>
                <span className="muted">{t("Publish this TXT record on the domain:")}</span> <code>{d.txt}</code>{" "}
                <button type="button" className="ghost" onClick={() => verify.mutate(d.name)}>
                  {t("Verify {domain}", { domain: d.name })}
                </button>
              </>
            )}
            <button type="button" className="ghost" aria-label={t("Remove domain {domain}", { domain: d.name })} onClick={() => remove.mutate(d.name)}>
              ×
            </button>
          </li>
        ))}
      </ul>
      <form onSubmit={submit}>
        <input aria-label={t("Email domain for {name}", { name: c.name })} placeholder="acme.com" value={domain} required onChange={(e) => setDomain(e.target.value)} />
        <label>
          <input type="checkbox" aria-label={t("Trusted (skip DNS check) for {name}", { name: c.name })} checked={trusted} onChange={(e) => setTrusted(e.target.checked)} />{" "}
          {t("Trusted (skip DNS check)")}
        </label>
        <button type="submit" className="ghost" aria-label={t("Add domain to {name}", { name: c.name })}>
          {t("Add domain")}
        </button>
      </form>
    </div>
  );
}
