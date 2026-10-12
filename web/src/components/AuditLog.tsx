import { useQuery } from "@tanstack/react-query";
import { api } from "../api";

const issueOf = (target: string) => target.split("/")[4] ?? "";

const LABELS: Record<string, (target: string) => string> = {
  "POST /api/v1/projects/{key}/issues": () => "Created an issue",
  "PATCH /api/v1/issues/{issueKey}": (t) => `Edited ${issueOf(t)}`,
  "POST /api/v1/issues/{issueKey}/transitions": (t) => `Moved ${issueOf(t)}`,
  "POST /api/v1/issues/{issueKey}/comments": (t) => `Commented on ${issueOf(t)}`,
  "POST /api/v1/issues/{issueKey}/rank": (t) => `Ranked ${issueOf(t)}`,
  "POST /api/v1/projects/{key}/members": () => "Changed project members",
  "PUT /api/v1/projects/{key}/workflow": () => "Changed the workflow",
  "POST /api/v1/projects/{key}/boards": () => "Created a board",
  "POST /api/v1/projects/{key}/sprints": () => "Created a sprint",
  "POST /api/v1/projects/{key}/import/jira": () => "Imported from Jira",
};

/** describeAction turns an API operation into plain words; unknown ones show the request. */
export function describeAction(action: string, target: string): string {
  const label = LABELS[action];
  return label ? label(target) : `${action.split(" ")[0]} ${target}`;
}

/** AuditLog shows project admins every change made in the project, refused attempts included. */
export function AuditLog({ projectKey }: { projectKey: string }) {
  const log = useQuery({ queryKey: ["audit", projectKey], queryFn: () => api.audit(projectKey) });
  if (log.isPending) return <p className="muted">Loading audit log…</p>;
  if (log.isError) return <p role="alert" className="error">{log.error.message}</p>;
  return (
    <section className="audit panel" aria-label="Audit log">
      <h2>Audit log</h2>
      <p className="muted">Every change made in this project, newest first. Only project admins see this.</p>
      <div className="table-scroll">
        <table>
          <thead>
            <tr>
              <th scope="col">When</th>
              <th scope="col">Who</th>
              <th scope="col">What</th>
              <th scope="col">Result</th>
            </tr>
          </thead>
          <tbody>
            {log.data.map((e, i) => (
              <tr key={`${e.at}-${i}`}>
                <td>
                  <time dateTime={e.at}>{new Date(e.at).toLocaleString()}</time>
                </td>
                <td>{e.actor_name}</td>
                <td title={`${e.action}\n${e.target}`}>{describeAction(e.action, e.target)}</td>
                <td className={e.status >= 400 ? "error" : "muted"}>{e.status >= 400 ? `Refused (${e.status})` : "OK"}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
