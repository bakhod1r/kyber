import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { notifyUseCases, type NotificationSettings } from "../application";
import { identityUseCases } from "../../identity/application";
import { useT } from "../../../shared/i18n/i18n";

const KEY = ["notification-settings"];

/** SettingsPage: where notifications go besides the in-app inbox (email, Telegram). */
export function SettingsPage() {
  const t = useT();
  const qc = useQueryClient();
  const [link, setLink] = useState<string | null>(null);
  const settings = useQuery({
    queryKey: KEY,
    queryFn: notifyUseCases.notificationSettings,
    // While the user is in Telegram pressing Start, check every 2 s whether the chat got connected.
    refetchInterval: (q) => (link && !q.state.data?.telegram_connected ? 2000 : false),
  });
  const save = useMutation({
    mutationFn: (patch: Partial<Pick<NotificationSettings, "email" | "telegram">>) => notifyUseCases.setNotificationSettings(patch),
    onSuccess: (s) => qc.setQueryData(KEY, s),
  });
  const connect = useMutation({
    mutationFn: identityUseCases.connectTelegram,
    onSuccess: (r) => {
      setLink(r.url);
      window.open(r.url, "_blank", "noopener");
    },
  });

  if (settings.isPending) return <p className="muted">{t("Loading settings…")}</p>;
  if (settings.isError) return <p role="alert" className="error">{settings.error.message}</p>;
  const s = settings.data;
  return (
    <section className="settings panel">
      <h2>{t("Notifications")}</h2>
      <p className="muted">{t("You always see notifications in Kyber. Choose where else they should reach you.")}</p>

      <h2>{t("Email")}</h2>
      {s.email_available ? (
        <label className="switch">
          <input type="checkbox" checked={s.email} onChange={(e) => save.mutate({ email: e.target.checked })} /> {t("Email me about assignments, comments and mentions")}
        </label>
      ) : (
        <p className="muted">{t("Email is not configured on this server (ask an administrator to set KYBER_SMTP_HOST).")}</p>
      )}

      <h2>{t("Telegram")}</h2>
      {!s.telegram_available && <p className="muted">{t("Telegram is not configured on this server.")}</p>}
      {s.telegram_available && (
        <>
          <p>
            {t("Status:")} <strong className={s.telegram_connected ? "ok" : "muted"}>{s.telegram_connected ? t("Connected") : t("Not connected")}</strong>
          </p>
          {s.telegram_connected ? (
            <label className="switch">
              <input type="checkbox" checked={s.telegram} onChange={(e) => save.mutate({ telegram: e.target.checked })} /> {t("Send my notifications to Telegram")}
            </label>
          ) : (
            <>
              <button type="button" onClick={() => connect.mutate()} disabled={connect.isPending}>
                {t("Connect Telegram")}
              </button>
              {link && (
                <p className="muted">
                  {t("Press")} <b>{t("Start")}</b> {t("in the bot. The link works once, for 10 minutes.")}{" "}
                  <a href={link} target="_blank" rel="noopener noreferrer">
                    {t("Open the bot again")}
                  </a>
                </p>
              )}
            </>
          )}
        </>
      )}
      {(save.isError || connect.isError) && <p role="alert" className="error">{(save.error ?? connect.error)?.message}</p>}
    </section>
  );
}
