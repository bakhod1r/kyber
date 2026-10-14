import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { type Comment, type Emoji, issueUseCases, REACTIONS } from "../application";
import { useT } from "../../../shared/i18n/i18n";

/** CommentReactions: emoji chips under a comment; clicking one toggles the caller's reaction. */
export function CommentReactions({ issueKey, comment }: { issueKey: string; comment: Comment }) {
  const t = useT();
  const qc = useQueryClient();
  const [picking, setPicking] = useState(false);
  const toggle = useMutation({
    mutationFn: ({ emoji, on }: { emoji: Emoji; on: boolean }) =>
      on ? issueUseCases.react(comment.id, emoji) : issueUseCases.unreact(comment.id, emoji),
    onSettled: () => qc.invalidateQueries({ queryKey: ["comments", issueKey] }),
  });
  const mine = (emoji: Emoji) => comment.reactions.some((r) => r.emoji === emoji && r.mine);
  const pick = (emoji: Emoji) => {
    setPicking(false);
    toggle.mutate({ emoji, on: !mine(emoji) });
  };

  return (
    <div className="reactions" role="group" aria-label={t("Reactions")}>
      {comment.reactions.map((r) => (
        <button
          key={r.emoji}
          type="button"
          className="reaction"
          aria-pressed={r.mine}
          aria-label={t("{emoji} reacted by {count}", { emoji: r.emoji, count: r.count })}
          disabled={toggle.isPending}
          onClick={() => toggle.mutate({ emoji: r.emoji, on: !r.mine })}
        >
          {r.emoji} {r.count}
        </button>
      ))}
      <button type="button" className="reaction ghost" aria-expanded={picking} aria-label={t("Add reaction")} onClick={() => setPicking(!picking)}>
        ☺+
      </button>
      {picking && (
        <span className="reaction-picker">
          {REACTIONS.map((emoji) => (
            <button key={emoji} type="button" className="ghost" aria-label={t("React with {emoji}", { emoji })} onClick={() => pick(emoji)}>
              {emoji}
            </button>
          ))}
        </span>
      )}
      {toggle.isError && (
        <span role="alert" className="error">
          {toggle.error.message}
        </span>
      )}
    </div>
  );
}
