import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { identityUseCases, type CreatedToken } from "../application";
import { msg, type MessageKey, useT } from "../../../shared/i18n/i18n";

const KEY = ["access-tokens"];

const lifetimes: { days: number; label: MessageKey }[] = [
  { days: 30, label: msg("30 days") },
  { days: 90, label: msg("90 days") },
  { days: 365, label: msg("1 year") },
  { days: 0, label: msg("No expiry") },
];

function day(iso: string | null, none: string): string {
  return iso ? new Date(iso).toLocaleDateString() : none;
}

/** AccessTokens: personal access tokens for scripts and AI assistants (MCP). */
export function AccessTokens() {
  const t = useT();
  const qc = useQueryClient();
  const [name, setName] = useState("");
  const [days, setDays] = useState(90);
  const [created, setCreated] = useState<CreatedToken | null>(null);
  const tokens = useQuery({ queryKey: KEY, queryFn: identityUseCases.tokens });
  const create = useMutation({
    mutationFn: () => identityUseCases.createToken(name, days),
    onSuccess: (t) => {
      setCreated(t);
      setName("");
    },
    onSettled: () => qc.invalidateQueries({ queryKey: KEY }),
  });
  const revoke = useMutation({
    mutationFn: identityUseCases.revokeToken,
    onSettled: () => qc.invalidateQueries({ queryKey: KEY }),
  });

  function submit(e: FormEvent) {
    e.preventDefault();
    create.mutate();
  }

  const error = create.error?.message || revoke.error?.message || tokens.error?.message;
  return (
    <section className="settings panel" aria-label={t("Access tokens")}>
      <h2>{t("Access tokens")}</h2>
      <p className="muted">
        {t("Tokens let scripts and AI assistants (MCP) act as you. Send one as")} <code>{t("Authorization: Bearer …")}</code>; {t("revoke it when you stop using it.")}
      </p>
      <form className="row" onSubmit={submit}>
        <input aria-label={t("Token name")} placeholder={t("e.g. Claude Code on my laptop")} value={name} onChange={(e) => setName(e.target.value)} />
        <select aria-label={t("Expires")} value={days} onChange={(e) => setDays(Number(e.target.value))}>
          {lifetimes.map((l) => (
            <option key={l.days} value={l.days}>
              {t(l.label)}
            </option>
          ))}
        </select>
        <button type="submit" disabled={create.isPending || name.trim() === ""}>
          {t("Create token")}
        </button>
      </form>
      {created && (
        <div className="notice" role="status">
          <p>
            {t("Copy")} <strong>{created.name}</strong> {t("now — it will not be shown again:")}
          </p>
          <code className="secret" aria-label={t("New token")}>
            {created.token}
          </code>
          <button type="button" className="ghost" onClick={() => void navigator.clipboard?.writeText(created.token)}>
            {t("Copy")}
          </button>
        </div>
      )}
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {tokens.data && tokens.data.length > 0 && (
        <table>
          <thead>
            <tr>
              <th>{t("Name")}</th>
              <th>{t("Created")}</th>
              <th>{t("Expires")}</th>
              <th>{t("Last used")}</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {tokens.data.map((tok) => (
              <tr key={tok.id}>
                <td>{tok.name}</td>
                <td>{day(tok.created_at, "")}</td>
                <td>{day(tok.expires_at, t("Never"))}</td>
                <td>{day(tok.last_used_at, t("Never"))}</td>
                <td>
                  <button type="button" className="ghost" aria-label={t("Revoke {name}", { name: tok.name })} disabled={revoke.isPending} onClick={() => revoke.mutate(tok.id)}>
                    {t("Revoke")}
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  );
}
