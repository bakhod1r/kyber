import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { issueUseCases, type LinkType } from "../application";
import { msg, type MessageKey, useT } from "../../../shared/i18n/i18n";

const TYPES: { id: LinkType; label: MessageKey }[] = [
  { id: "blocks", label: msg("blocks") },
  { id: "relates", label: msg("relates to") },
  { id: "duplicates", label: msg("duplicates") },
  { id: "clones", label: msg("clones") },
];

/** IssueLinks: blocks / relates / duplicates / clones, read from this issue's side (KYB-S45). */
export function IssueLinks({ issueKey, onOpen }: { issueKey: string; onOpen?: (key: string) => void }) {
  const t = useT();
  const qc = useQueryClient();
  const links = useQuery({ queryKey: ["links", issueKey], queryFn: () => issueUseCases.links(issueKey) });
  const [type, setType] = useState<LinkType>("blocks");
  const [to, setTo] = useState("");
  const refresh = () => qc.invalidateQueries({ queryKey: ["links"] });
  const add = useMutation({
    mutationFn: () => issueUseCases.link(issueKey, type, to.trim().toUpperCase()),
    onSuccess: () => {
      setTo("");
      return refresh();
    },
  });
  const remove = useMutation({ mutationFn: (id: string) => issueUseCases.unlink(id), onSuccess: refresh });
  function submit(e: FormEvent) {
    e.preventDefault();
    if (to.trim()) add.mutate();
  }
  return (
    <section className="links" aria-label={t("Links")}>
      <h3>{t("Links")}</h3>
      {links.isError && <p className="error">{links.error.message}</p>}
      {links.data?.length === 0 && <p className="muted">{t("No linked issues.")}</p>}
      <ul aria-label={t("Linked issues")} className="child-list">
        {links.data?.map((l) => (
          <li key={l.id}>
            <span className="muted">{l.phrase}</span>
            <button type="button" className="link" onClick={() => onOpen?.(l.issue.key)}>
              {l.issue.key} {l.issue.title}
            </button>
            <span className={`status status-${l.issue.status}`}>{l.issue.status.replace("_", " ")}</span>
            <button type="button" className="ghost" aria-label={t("Remove link to {key}", { key: l.issue.key })} onClick={() => remove.mutate(l.id)}>
              ×
            </button>
          </li>
        ))}
      </ul>
      <form className="inline-form" onSubmit={submit}>
        <select aria-label={t("Link type")} value={type} onChange={(e) => setType(e.target.value as LinkType)}>
          {TYPES.map((lt) => (
            <option key={lt.id} value={lt.id}>
              {t(lt.label)}
            </option>
          ))}
        </select>
        <input aria-label={t("Issue key to link")} placeholder={t("KYB-12")} value={to} onChange={(e) => setTo(e.target.value)} />
        <button type="submit" disabled={add.isPending}>
          {t("Link")}
        </button>
      </form>
      {(add.isError || remove.isError) && <p role="alert" className="error">{(add.error ?? remove.error)?.message}</p>}
    </section>
  );
}
