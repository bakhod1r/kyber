import { useQuery } from "@tanstack/react-query";
import { Link } from "react-router-dom";
import { notifyUseCases } from "../application";

/** My mentions in chat, unread first (S64); opening one opens its conversation. */
export function ChatMentions() {
  const inbox = useQuery({ queryKey: ["notifications"], queryFn: notifyUseCases.notifications, refetchInterval: 10_000 });
  const mentions = (inbox.data?.items ?? []).filter((n) => n.conversation_id);
  const sorted = [...mentions.filter((n) => !n.read), ...mentions.filter((n) => n.read)];
  return (
    <>
      <h2>Mentions</h2>
      {sorted.length === 0 ? <p className="muted">No mentions yet.</p> : (
        <ul aria-label="Mentions" className="chat-mentions">
          {sorted.map((n) => (
            <li key={n.id} className={n.read ? "" : "unread"}>
              <Link to={`/chat/${n.conversation_id}`}>
                <b>{n.actor_name}</b> in {n.conversation_name ? `#${n.conversation_name}` : "a direct message"}
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
