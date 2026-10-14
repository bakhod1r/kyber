import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { initials, issueUseCases } from "../application";
import { useT } from "../../../shared/i18n/i18n";

/** How often the open issue panel says "I'm here" (the server keeps a heartbeat for 45 s). */
export const PRESENCE_EVERY_MS = 20_000;

/** IssuePresence: "Also viewing" — the other people who have this issue open right now. */
export function IssuePresence({ issueKey, me }: { issueKey: string; me?: string }) {
  const t = useT();
  const qc = useQueryClient();
  const key = ["presence", issueKey];
  // Live hints (presence.changed) invalidate every query; the interval covers people leaving.
  const viewers = useQuery({ queryKey: key, queryFn: () => issueUseCases.viewers(issueKey), refetchInterval: PRESENCE_EVERY_MS });
  useEffect(() => {
    let live = true;
    const beat = () =>
      issueUseCases.heartbeat(issueKey).then(
        () => {
          if (live) void qc.invalidateQueries({ queryKey: ["presence", issueKey] });
        },
        () => undefined, // presence is best effort
      );
    void beat();
    const timer = setInterval(() => void beat(), PRESENCE_EVERY_MS);
    return () => {
      live = false;
      clearInterval(timer);
    };
  }, [issueKey, qc]);

  const others = (viewers.data ?? []).filter((v) => v.user_id !== me);
  if (others.length === 0) return null;
  return (
    <p className="presence muted" aria-label={t("Also viewing")}>
      {t("Also viewing:")}{" "}
      {others.map((v) => (
        <span key={v.user_id} className="avatar" title={v.name} aria-label={v.name}>
          {initials(v.name)}
        </span>
      ))}
    </p>
  );
}
