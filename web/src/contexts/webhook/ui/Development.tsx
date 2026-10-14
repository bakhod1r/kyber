import { useQuery } from "@tanstack/react-query";
import { useT } from "../../../shared/i18n/i18n";
import { shortRef, webhookUseCases } from "../application";

/** Development: commits and pull requests that mention the issue (from the GitHub integration). */
export function Development({ issueKey }: { issueKey: string }) {
  const t = useT();
  const links = useQuery({ queryKey: ["development", issueKey], queryFn: () => webhookUseCases.development(issueKey), retry: false });
  if (!links.data || links.data.length === 0) return null; // nothing linked (or not visible): no empty section
  return (
    <section className="development" aria-label={t("Development")}>
      <h3>{t("Development")}</h3>
      <ul className="child-list">
        {links.data.map((l) => (
          <li key={`${l.kind}/${l.repo}/${l.ref}`}>
            <span className="muted">{l.kind === "commit" ? t("Commit") : t("Pull request")}</span>{" "}
            <a href={l.url} target="_blank" rel="noopener noreferrer">
              {l.repo} {shortRef(l)}
            </a>{" "}
            {l.title}
            {l.state && <span className={`badge ${l.state}`}> {t(l.state === "merged" ? "Merged" : l.state === "closed" ? "Closed" : "Open")}</span>}
          </li>
        ))}
      </ul>
    </section>
  );
}
