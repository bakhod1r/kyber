import { formatDateTime } from "../../../shared/i18n/labels";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { ApiError } from "../../../shared/http";
import { useT } from "../../../shared/i18n/i18n";
import { parseEvents, WEBHOOK_EVENTS, webhookUseCases, type Webhook } from "../application";

/** Webhooks: a project's outgoing webhooks (project admins). Secrets are shown once, on creation. */
export function Webhooks({ projectKey }: { projectKey: string }) {
  const t = useT();
  const qc = useQueryClient();
  const key = ["webhooks", projectKey];
  const [url, setUrl] = useState("");
  const [events, setEvents] = useState("");
  const [secret, setSecret] = useState<{ url: string; secret: string } | null>(null);
  const [open, setOpen] = useState<string | null>(null);
  const hooks = useQuery({ queryKey: key, queryFn: () => webhookUseCases.webhooks(projectKey), retry: false });
  const refresh = () => qc.invalidateQueries({ queryKey: key });
  const create = useMutation({
    mutationFn: () => webhookUseCases.createWebhook(projectKey, { url: url.trim(), events: parseEvents(events) }),
    onSuccess: (w) => {
      setSecret({ url: w.url, secret: w.secret ?? "" });
      setUrl("");
      setEvents("");
      return refresh();
    },
  });
  const toggle = useMutation({
    mutationFn: (w: Webhook) => webhookUseCases.updateWebhook(projectKey, w.id, { url: w.url, events: w.events, active: !w.active }),
    onSuccess: refresh,
  });
  const remove = useMutation({ mutationFn: (id: string) => webhookUseCases.deleteWebhook(projectKey, id), onSuccess: refresh });

  if (hooks.isPending) return <p className="muted">{t("Loading…")}</p>;
  if (hooks.isError) {
    const forbidden = hooks.error instanceof ApiError && hooks.error.status === 403;
    return <p className="muted">{forbidden ? t("Only project admins can manage webhooks.") : hooks.error.message}</p>;
  }
  const submit = (e: FormEvent) => {
    e.preventDefault();
    create.mutate();
  };
  const error = create.error ?? toggle.error ?? remove.error;
  return (
    <section className="webhooks panel" aria-labelledby="webhooks-title">
      <h2 id="webhooks-title">{t("Webhooks")}</h2>
      <p className="muted">
        {t("Kyber POSTs this project's events as JSON, signed with X-Kyber-Signature (HMAC-SHA256 of the body). Payloads carry identifiers only; fetch details through the API.")}
      </p>
      {hooks.data.length === 0 ? (
        <p className="muted">{t("No webhooks yet.")}</p>
      ) : (
        <ul aria-label={t("Webhooks")} className="child-list">
          {hooks.data.map((w) => (
            <li key={w.id}>
              <span>
                <code>{w.url}</code> · {w.events.join(", ")}
              </span>
              <label>
                <input type="checkbox" checked={w.active} onChange={() => toggle.mutate(w)} /> {t("Active")}
              </label>
              <button type="button" className="ghost" onClick={() => setOpen(open === w.id ? null : w.id)}>
                {t("Deliveries")}
              </button>
              <button type="button" className="ghost" aria-label={t("Delete webhook {url}", { url: w.url })} onClick={() => remove.mutate(w.id)}>
                {t("Delete")}
              </button>
              {open === w.id && <Deliveries projectKey={projectKey} id={w.id} />}
            </li>
          ))}
        </ul>
      )}
      <form onSubmit={submit} aria-label={t("Add webhook")}>
        <label>
          {t("Payload URL")}
          <input type="url" required value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://ci.example.com/kyber" />
        </label>
        <label>
          {t("Events (comma-separated; empty = all)")}
          <input value={events} onChange={(e) => setEvents(e.target.value)} placeholder={WEBHOOK_EVENTS.slice(0, 2).join(", ")} list="webhook-events" />
        </label>
        <datalist id="webhook-events">
          {WEBHOOK_EVENTS.map((e) => (
            <option key={e} value={e} />
          ))}
        </datalist>
        <button type="submit" disabled={create.isPending}>
          {t("Add webhook")}
        </button>
      </form>
      {secret && (
        <p role="status">
          {t("Secret for {url} — copy it now, it is not shown again:", { url: secret.url })} <code>{secret.secret}</code>
        </p>
      )}
      {error && (
        <p role="alert" className="error">
          {error.message}
        </p>
      )}
    </section>
  );
}

function Deliveries({ projectKey, id }: { projectKey: string; id: string }) {
  const t = useT();
  const list = useQuery({ queryKey: ["webhook-deliveries", projectKey, id], queryFn: () => webhookUseCases.deliveries(projectKey, id) });
  if (list.isPending) return <p className="muted">{t("Loading…")}</p>;
  if (list.isError) return <p className="error">{list.error.message}</p>;
  if (list.data.length === 0) return <p className="muted">{t("No deliveries yet.")}</p>;
  return (
    <table aria-label={t("Recent deliveries")}>
      <thead>
        <tr>
          <th scope="col">{t("Event")}</th>
          <th scope="col">{t("Status")}</th>
          <th scope="col">{t("Attempts")}</th>
          <th scope="col">{t("Response")}</th>
          <th scope="col">{t("Time")}</th>
        </tr>
      </thead>
      <tbody>
        {list.data.map((d) => (
          <tr key={d.id}>
            <td>{d.event}</td>
            <td>{t(d.status === "delivered" ? "Delivered" : d.status === "failed" ? "Failed" : "Pending")}</td>
            <td>{d.attempts}</td>
            <td>{d.last_status ? d.last_status : d.last_error || "—"}</td>
            <td>{formatDateTime(d.created_at)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
