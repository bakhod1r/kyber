import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState, type FormEvent } from "react";
import { identityUseCases, type CreatedToken } from "../application";

const KEY = ["access-tokens"];

const lifetimes = [
  { days: 30, label: "30 days" },
  { days: 90, label: "90 days" },
  { days: 365, label: "1 year" },
  { days: 0, label: "No expiry" },
];

function day(iso: string | null, none: string): string {
  return iso ? new Date(iso).toLocaleDateString() : none;
}

/** AccessTokens: personal access tokens for scripts and AI assistants (MCP). */
export function AccessTokens() {
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
    <section className="settings panel" aria-label="Access tokens">
      <h2>Access tokens</h2>
      <p className="muted">
        Tokens let scripts and AI assistants (MCP) act as you. Send one as <code>Authorization: Bearer …</code>; revoke it when you stop using it.
      </p>
      <form className="row" onSubmit={submit}>
        <input aria-label="Token name" placeholder="e.g. Claude Code on my laptop" value={name} onChange={(e) => setName(e.target.value)} />
        <select aria-label="Expires" value={days} onChange={(e) => setDays(Number(e.target.value))}>
          {lifetimes.map((l) => (
            <option key={l.days} value={l.days}>
              {l.label}
            </option>
          ))}
        </select>
        <button type="submit" disabled={create.isPending || name.trim() === ""}>
          Create token
        </button>
      </form>
      {created && (
        <div className="notice" role="status">
          <p>
            Copy <strong>{created.name}</strong> now — it will not be shown again:
          </p>
          <code className="secret" aria-label="New token">
            {created.token}
          </code>
          <button type="button" className="ghost" onClick={() => void navigator.clipboard?.writeText(created.token)}>
            Copy
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
              <th>Name</th>
              <th>Created</th>
              <th>Expires</th>
              <th>Last used</th>
              <th />
            </tr>
          </thead>
          <tbody>
            {tokens.data.map((t) => (
              <tr key={t.id}>
                <td>{t.name}</td>
                <td>{day(t.created_at, "")}</td>
                <td>{day(t.expires_at, "Never")}</td>
                <td>{day(t.last_used_at, "Never")}</td>
                <td>
                  <button type="button" className="ghost" aria-label={`Revoke ${t.name}`} disabled={revoke.isPending} onClick={() => revoke.mutate(t.id)}>
                    Revoke
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
