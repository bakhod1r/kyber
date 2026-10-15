import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { notifyUseCases } from "../application";
import { useT } from "../../../shared/i18n/i18n";

/** My mentions in chat, unread first (S64); opening one opens its conversation. */
export function ChatMentions() {
  const t = useT();
  const inbox = useQuery({ queryKey: ["notifications"], queryFn: notifyUseCases.notifications, refetchInterval: 10_000 });
  const mentions = (inbox.data?.items ?? []).filter((n) => n.conversation_id && n.kind === "mentioned");
  const sorted = [...mentions.filter((n) => !n.read), ...mentions.filter((n) => n.read)];
  return (
    <>
      <h2>{t("Mentions")}</h2>
      {sorted.length === 0 ? <p className="muted">{t("No mentions yet.")}</p> : (
        <ul aria-label={t("Mentions")} className="chat-mentions">
          {sorted.map((n) => (
            <li key={n.id} className={n.read ? "" : "unread"}>
              <Link to={`/chat/${n.conversation_id}`}>
                {t("{name} in {where}", { name: n.actor_name, where: n.conversation_name ? `#${n.conversation_name}` : t("a direct message") })}
              </Link>{" "}
              <time className="muted" dateTime={n.created_at}>{new Date(n.created_at).toLocaleString()}</time>
              <p>{n.excerpt}</p>
            </li>
          ))}
        </ul>
      )}
    </>
  );
}
