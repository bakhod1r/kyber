import { formatDateTime } from "../../../shared/i18n/labels";
import { useQuery } from "@tanstack/react-query";
import { auditUseCases } from "../application";
import { type Translate, translate, useT } from "../../../shared/i18n/i18n";

const issueOf = (target: string) => target.split("/")[4] ?? "";

const LABELS: Record<string, (tr: Translate, target: string) => string> = {
  "POST /api/v1/projects/{key}/issues": (tr) => tr("Created an issue"),
  "PATCH /api/v1/issues/{issueKey}": (tr, t) => tr("Edited {key}", { key: issueOf(t) }),
  "POST /api/v1/issues/{issueKey}/transitions": (tr, t) => tr("Moved {key}", { key: issueOf(t) }),
  "POST /api/v1/issues/{issueKey}/comments": (tr, t) => tr("Commented on {key}", { key: issueOf(t) }),
  "POST /api/v1/issues/{issueKey}/rank": (tr, t) => tr("Ranked {key}", { key: issueOf(t) }),
  "POST /api/v1/projects/{key}/members": (tr) => tr("Changed project members"),
  "PUT /api/v1/projects/{key}/workflow": (tr) => tr("Changed the workflow"),
  "POST /api/v1/projects/{key}/boards": (tr) => tr("Created a board"),
  "POST /api/v1/projects/{key}/sprints": (tr) => tr("Created a sprint"),
  "POST /api/v1/projects/{key}/import/jira": (tr) => tr("Imported from Jira"),
  "POST /api/v1/sprints/{id}/start": (tr) => tr("Started a sprint"),
  "POST /api/v1/sprints/{id}/complete": (tr) => tr("Completed a sprint"),
  "PUT /api/v1/boards/{id}": (tr) => tr("Changed a board"),
  "DELETE /api/v1/boards/{id}": (tr) => tr("Deleted a board"),
};

const english: Translate = (k, v) => translate("en", k, v);

/** describeAction turns an API operation into plain words; unknown ones show the request. */
export function describeAction(action: string, target: string, tr: Translate = english): string {
  const label = LABELS[action];
  return label ? label(tr, target) : `${action.split(" ")[0]} ${target}`;
}

/** AuditLog shows project admins every change made in the project, refused attempts included. */
export function AuditLog({ projectKey }: { projectKey: string }) {
  const t = useT();
  const log = useQuery({ queryKey: ["audit", projectKey], queryFn: () => auditUseCases.audit(projectKey) });
  if (log.isPending) return <p className="muted">{t("Loading audit log…")}</p>;
  if (log.isError) return <p role="alert" className="error">{log.error.message}</p>;
  return (
    <section className="audit panel" aria-label={t("Audit log")}>
      <h2>{t("Audit log")}</h2>
      <p className="muted">{t("Every change made in this project, newest first. Only project admins see this.")}</p>
      <div className="table-scroll">
        <table>
          <thead>
            <tr>
              <th scope="col">{t("When")}</th>
              <th scope="col">{t("Who")}</th>
              <th scope="col">{t("What")}</th>
              <th scope="col">{t("Result")}</th>
            </tr>
          </thead>
          <tbody>
            {log.data.map((e, i) => (
              <tr key={`${e.at}-${i}`}>
                <td>
                  <time dateTime={e.at}>{formatDateTime(e.at)}</time>
                </td>
                <td>{e.actor_name}</td>
                <td title={`${e.action}\n${e.target}`}>{describeAction(e.action, e.target, t)}</td>
                <td className={e.status >= 400 ? "error" : "muted"}>{e.status >= 400 ? t("Refused ({status})", { status: e.status }) : t("OK")}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
