import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { ApiError } from "../../../shared/http";
import { notifyUseCases } from "../application";
import { useT } from "../../../shared/i18n/i18n";

/** TelegramGroups: a project's Telegram groups, which get new issues, status changes and sprints. */
export function TelegramGroups({ projectKey }: { projectKey: string }) {
  const t = useT();
  const qc = useQueryClient();
  const key = ["telegram-groups", projectKey];
  const [link, setLink] = useState<string | null>(null);
  const groups = useQuery({
    queryKey: key,
    queryFn: () => notifyUseCases.telegramGroups(projectKey),
    retry: false,
    refetchInterval: link ? 2000 : false, // while the admin is picking a group in Telegram
  });
  const connect = useMutation({
    mutationFn: () => notifyUseCases.telegramGroupLink(projectKey),
    onSuccess: (r) => {
      setLink(r.url);
      window.open(r.url, "_blank", "noopener");
    },
  });
  const unlink = useMutation({
    mutationFn: (chat: string) => notifyUseCases.unlinkTelegramGroup(projectKey, chat),
    onSuccess: () => qc.invalidateQueries({ queryKey: key }),
  });

  if (groups.isPending) return <p className="muted">{t("Loading…")}</p>;
  if (groups.isError) {
    const forbidden = groups.error instanceof ApiError && groups.error.status === 403;
    return <p className="muted">{forbidden ? t("Only project admins can connect Telegram groups.") : groups.error.message}</p>;
  }
  return (
    <section className="telegram-groups panel" aria-label={t("Telegram groups")}>
      <h2>{t("Telegram groups")}</h2>
      <p className="muted">{t("Connected groups get this project's new issues, status changes and sprint starts and completions.")}</p>
      {groups.data.length === 0 ? (
        <p className="muted">{t("No Telegram groups yet.")}</p>
      ) : (
        <ul aria-label={t("Connected groups")} className="child-list">
          {groups.data.map((g) => (
            <li key={g.chat_id}>
              <span>{g.title || g.chat_id}</span>
              <button type="button" className="ghost" aria-label={t("Disconnect {name}", { name: g.title || g.chat_id })} onClick={() => unlink.mutate(g.chat_id)}>
                {t("Disconnect")}
              </button>
            </li>
          ))}
        </ul>
      )}
      <button type="button" onClick={() => connect.mutate()} disabled={connect.isPending}>
        {t("Connect a Telegram group")}
      </button>
      {link && (
        <p className="muted">
          Choose the group in Telegram; the bot joins it and confirms. The link works once, for 10 minutes.{" "}
          <a href={link} target="_blank" rel="noopener noreferrer">
            {t("Open again")}
          </a>
        </p>
      )}
      {(connect.isError || unlink.isError) && <p role="alert" className="error">{(connect.error ?? unlink.error)?.message}</p>}
    </section>
  );
}
