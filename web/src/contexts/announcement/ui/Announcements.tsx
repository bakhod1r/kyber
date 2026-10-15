import { formatDateTime } from "../../../shared/i18n/labels";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { Markdown } from "../../../shared/ui/Markdown";
import { announcementUseCases } from "../application";
import { useT } from "../../../shared/i18n/i18n";

/** Announcements of a project, or of the whole workspace (projectKey null): admins publish, everyone
 * in it is notified and reads the feed here. */
export function Announcements({ projectKey }: { projectKey: string | null }) {
  const t = useT();
  const qc = useQueryClient();
  const queryKey = ["announcements", projectKey ?? ""];
  const feed = useQuery({ queryKey, queryFn: () => announcementUseCases.announcements(projectKey) });
  const [writing, setWriting] = useState(false);
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const publish = useMutation({
    mutationFn: () => announcementUseCases.publish(projectKey, { title, body }),
    onSuccess: () => {
      setTitle("");
      setBody("");
      setWriting(false);
      return qc.invalidateQueries({ queryKey });
    },
  });
  const submit = (e: FormEvent) => {
    e.preventDefault();
    publish.mutate();
  };
  const label = projectKey ? t("Announcements") : t("Workspace announcements");
  return (
    <section className="announcements" aria-label={label}>
      <div className="row between">
        <h2>{label}</h2>
        {!writing && (
          <button type="button" onClick={() => setWriting(true)}>
            {t("New announcement")}
          </button>
        )}
      </div>
      {writing && (
        <form className="announcement-form" onSubmit={submit}>
          <label>
            {t("Headline")}
            <input value={title} onChange={(e) => setTitle(e.target.value)} required maxLength={120} />
          </label>
          <label>
            {t("Message (Markdown)")}
            <textarea rows={5} value={body} onChange={(e) => setBody(e.target.value)} maxLength={10000} />
          </label>
          {publish.isError && (
            <p role="alert" className="error">
              {publish.error.message}
            </p>
          )}
          <div className="row">
            <button type="submit" disabled={publish.isPending}>
              {t("Publish to everyone")}
            </button>
            <button type="button" className="ghost" onClick={() => setWriting(false)}>
              {t("Cancel")}
            </button>
          </div>
        </form>
      )}
      {feed.isError && (
        <p role="alert" className="error">
          {(feed.error as Error).message}
        </p>
      )}
      {feed.data?.length === 0 && <p className="muted">{t("No announcements yet.")}</p>}
      <ul className="announcement-feed">
        {feed.data?.map((a) => (
          <li key={a.id}>
            <article aria-label={a.title}>
              <h3>📣 {a.title}</h3>
              <time className="muted" dateTime={a.created_at}>
                {formatDateTime(a.created_at)}
              </time>
              {a.body && <Markdown source={a.body} />}
            </article>
          </li>
        ))}
      </ul>
    </section>
  );
}
