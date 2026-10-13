import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";
import { formatSize, issueUseCases, maxAttachmentSize } from "../application";

/** IssueAttachments: files on the issue — upload, open, remove (C5). */
export function IssueAttachments({ issueKey }: { issueKey: string }) {
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
      setTooBig(`${file.name} is ${formatSize(file.size)}; the limit is ${formatSize(maxAttachmentSize)}.`);
      return;
    }
    setTooBig("");
    attach.mutate(file);
  }

  const error = tooBig || attach.error?.message || remove.error?.message;
  return (
    <section className="attachments" aria-label="Attachments">
      <div className="row">
        <h4>Attachments</h4>
        <label className="ghost button">
          {attach.isPending ? "Uploading…" : "Attach file"}
          <input ref={input} type="file" hidden disabled={attach.isPending} aria-label="Attach file" onChange={(e) => pick(e.target.files)} />
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
              <button type="button" className="ghost" aria-label={`Remove ${a.name}`} disabled={remove.isPending} onClick={() => remove.mutate(a.id)}>
                ×
              </button>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
