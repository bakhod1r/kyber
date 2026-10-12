import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { ApiError, api } from "../api";

/** TelegramGroups: a project's Telegram groups, which get new issues, status changes and sprints. */
export function TelegramGroups({ projectKey }: { projectKey: string }) {
  const qc = useQueryClient();
  const key = ["telegram-groups", projectKey];
  const [link, setLink] = useState<string | null>(null);
  const groups = useQuery({
    queryKey: key,
    queryFn: () => api.telegramGroups(projectKey),
    retry: false,
    refetchInterval: link ? 2000 : false, // while the admin is picking a group in Telegram
  });
  const connect = useMutation({
    mutationFn: () => api.telegramGroupLink(projectKey),
    onSuccess: (r) => {
      setLink(r.url);
      window.open(r.url, "_blank", "noopener");
    },
  });
  const unlink = useMutation({
    mutationFn: (chat: string) => api.unlinkTelegramGroup(projectKey, chat),
    onSuccess: () => qc.invalidateQueries({ queryKey: key }),
  });

  if (groups.isPending) return <p className="muted">Loading…</p>;
  if (groups.isError) {
    const forbidden = groups.error instanceof ApiError && groups.error.status === 403;
    return <p className="muted">{forbidden ? "Only project admins can connect Telegram groups." : groups.error.message}</p>;
  }
  return (
    <section className="telegram-groups panel" aria-label="Telegram groups">
      <h2>Telegram groups</h2>
      <p className="muted">Connected groups get this project's new issues, status changes and sprint starts and completions.</p>
      {groups.data.length === 0 ? (
        <p className="muted">No Telegram groups yet.</p>
      ) : (
        <ul aria-label="Connected groups" className="child-list">
          {groups.data.map((g) => (
            <li key={g.chat_id}>
              <span>{g.title || g.chat_id}</span>
              <button type="button" className="ghost" aria-label={`Disconnect ${g.title || g.chat_id}`} onClick={() => unlink.mutate(g.chat_id)}>
                Disconnect
              </button>
            </li>
          ))}
        </ul>
      )}
      <button type="button" onClick={() => connect.mutate()} disabled={connect.isPending}>
        Connect a Telegram group
      </button>
      {link && (
        <p className="muted">
          Choose the group in Telegram; the bot joins it and confirms. The link works once, for 10 minutes.{" "}
          <a href={link} target="_blank" rel="noopener noreferrer">
            Open again
          </a>
        </p>
      )}
      {(connect.isError || unlink.isError) && <p role="alert" className="error">{(connect.error ?? unlink.error)?.message}</p>}
    </section>
  );
}
