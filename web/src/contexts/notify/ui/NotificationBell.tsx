import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { notificationTarget, notifyUseCases, type Notification } from "../application";
import { msg, type MessageKey, useT } from "../../../shared/i18n/i18n";

const VERB: Record<Notification["kind"], MessageKey> = {
  assigned: msg("assigned you"),
  commented: msg("commented on"),
  mentioned: msg("mentioned you on"),
  transitioned: msg("changed the status of"),
  announcement: msg("announced"),
  meeting: msg("📅"),
};

export function NotificationBell() {
  const t = useT();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);
  const inbox = useQuery({ queryKey: ["notifications"], queryFn: notifyUseCases.notifications, refetchInterval: 30_000 });
  const unread = inbox.data?.unread ?? 0;

  const refresh = () => qc.invalidateQueries({ queryKey: ["notifications"] });
  const markRead = useMutation({ mutationFn: notifyUseCases.markNotificationRead, onSettled: refresh });
  const markAll = useMutation({ mutationFn: notifyUseCases.markAllNotificationsRead, onSettled: refresh });

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setOpen(false);
    const onClick = (e: MouseEvent) => ref.current && !ref.current.contains(e.target as Node) && setOpen(false);
    document.addEventListener("keydown", onKey);
    document.addEventListener("mousedown", onClick);
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("mousedown", onClick);
    };
  }, [open]);

  function openIssue(n: Notification) {
    if (!n.read) markRead.mutate(n.id);
    setOpen(false);
    navigate(notificationTarget(n));
  }

  return (
    <div className="bell" ref={ref}>
      <button
        type="button"
        className="ghost bell-button"
        aria-label={unread > 0 ? t("Notifications ({n} unread)", { n: unread }) : t("Notifications")}
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
      >
        <svg width="18" height="18" viewBox="0 0 24 24" aria-hidden="true">
          <path fill="currentColor" d="M12 22a2.5 2.5 0 0 0 2.45-2h-4.9A2.5 2.5 0 0 0 12 22Zm7-6V11a7 7 0 0 0-5.5-6.84V3a1.5 1.5 0 0 0-3 0v1.16A7 7 0 0 0 5 11v5l-2 2v1h18v-1l-2-2Z" />
        </svg>
        {unread > 0 && <span className="badge">{unread > 99 ? "99+" : unread}</span>}
      </button>
      {open && (
        <div role="dialog" aria-label={t("Notifications")} className="panel bell-panel">
          <header>
            <h2>{t("Notifications")}</h2>
            {unread > 0 && (
              <button type="button" className="ghost" onClick={() => markAll.mutate()}>
                {t("Mark all as read")}
              </button>
            )}
          </header>
          {inbox.data?.items.length === 0 && <p className="muted">{t("You're all caught up.")}</p>}
          <ul>
            {inbox.data?.items.map((n) => (
              <li key={n.id}>
                <button type="button" className={`note${n.read ? "" : " unread"}`} onClick={() => openIssue(n)}>
                  <span className="note-line">
                    <strong>{n.actor_name}</strong> {t(VERB[n.kind])} <span className="key">{n.kind === "announcement" || n.kind === "meeting" ? n.project_key || (n.kind === "meeting" ? n.excerpt : "📣") : n.issue_key}</span>
                  </span>
                  <span className="note-title">{n.issue_title}</span>
                  {n.excerpt && <span className="note-excerpt">{n.excerpt}</span>}
                  <time dateTime={n.created_at}>{new Date(n.created_at).toLocaleString()}</time>
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
