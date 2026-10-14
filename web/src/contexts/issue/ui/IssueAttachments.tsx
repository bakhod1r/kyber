import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";
import { formatSize, issueUseCases, maxAttachmentSize } from "../application";
import { useT } from "../../../shared/i18n/i18n";

/** IssueAttachments: files on the issue — upload, open, remove (C5). */
export function IssueAttachments({ issueKey }: { issueKey: string }) {
  const t = useT();
  const qc = useQueryClient();
  const key = ["attachments", issueKey];
  const input = useRef<HTMLInputElement>(null);
  const [tooBig, setTooBig] = useState("");
  const list = useQuery({ queryKey: key, queryFn: () => issueUseCases.attachments(issueKey) });
  const attach = useMutation({
    mutationFn: (file: File) => issueUseCases.attach(issueKey, file),
    onSettled: () => qc.invalidateQueries({ queryKey: key }),
  });
  const remove = useMutation({
    mutationFn: (id: string) => issueUseCases.deleteAttachment(id),
    onSettled: () => qc.invalidateQueries({ queryKey: key }),
  });

  function pick(files: FileList | null) {
    const file = files?.[0];
    if (input.current) input.current.value = "";
    if (!file) return;
    if (file.size > maxAttachmentSize) {
      setTooBig(t("{name} is {size}; the limit is {limit}.", { name: file.name, size: formatSize(file.size), limit: formatSize(maxAttachmentSize) }));
      return;
    }
    setTooBig("");
    attach.mutate(file);
  }

  const error = tooBig || attach.error?.message || remove.error?.message;
  return (
    <section className="attachments" aria-label={t("Attachments")}>
      <div className="row">
        <h4>{t("Attachments")}</h4>
        <label className="ghost button">
          {attach.isPending ? t("Uploading…") : t("Attach file")}
          <input ref={input} type="file" hidden disabled={attach.isPending} aria-label={t("Attach file")} onChange={(e) => pick(e.target.files)} />
        </label>
      </div>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {list.data && list.data.length > 0 && (
        <ul className="attachment-list">
          {list.data.map((a) => (
            <li key={a.id} className="row">
              {a.inline && a.content_type.startsWith("image/") && <img src={a.url} alt="" className="attachment-thumb" loading="lazy" />}
              <a href={a.inline ? a.url : `${a.url}?download`} target={a.inline ? "_blank" : undefined} rel="noopener noreferrer">
                {a.name}
              </a>
              <span className="muted">{formatSize(a.size)}</span>
              <button type="button" className="ghost" aria-label={t("Remove {name}", { name: a.name })} disabled={remove.isPending} onClick={() => remove.mutate(a.id)}>
                ×
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
