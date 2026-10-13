// ProjectPage composes the project views of every bounded context (app shell / composition root).
import { useQuery } from "@tanstack/react-query";
import { NavLink, useParams, useSearchParams } from "react-router-dom";
import { projectUseCases } from "../contexts/project/application";
import { Members } from "../contexts/project/ui/Members";
import { IssueFocus } from "../contexts/focus/ui/Focus";
import { Backlog } from "../contexts/agile/ui/Backlog";
import { Board } from "../contexts/agile/ui/Board";
import { AuditLog } from "../contexts/audit/ui/AuditLog";
import { IssuePanel } from "../contexts/issue/ui/IssuePanel";
import { Releases } from "../contexts/agile/ui/Releases";
import { PermissionScheme } from "../contexts/access/ui/PermissionScheme";
import { TelegramGroups } from "../contexts/notify/ui/TelegramGroups";
import { WorkflowEditor } from "../contexts/issue/ui/WorkflowEditor";
import { JiraImport } from "../contexts/importer/ui/ImportPage";
import { Reports } from "../contexts/insights/ui/ReportsPage";

export function ProjectPage({ view = "board" }: { view?: "board" | "backlog" | "reports" | "import" | "workflow" | "audit" | "notifications" | "permissions" | "releases" }) {
  const key = useParams<{ key: string }>().key ?? "";
  const projects = useQuery({ queryKey: ["projects"], queryFn: projectUseCases.projects });
  const project = projects.data?.find((p) => p.key === key);
  // The open issue lives in the URL (?issue=KEY-1), so notifications and links can deep-link to it.
  const [params, setParams] = useSearchParams();
  const open = params.get("issue");
  const setOpen = (issueKey: string | null) =>
    setParams((p) => {
      if (issueKey) p.set("issue", issueKey);
      else p.delete("issue");
      return p;
    });
  return (
    <div className="project">
      <h1>
        <span className="key">{key}</span> {project?.name ?? ""}
      </h1>
      <nav className="tabs" aria-label="Project views">
        <NavLink to={`/projects/${encodeURIComponent(key)}`} end>
          Board
        </NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/backlog`}>Backlog</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/reports`}>Reports</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/releases`}>Releases</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/import`}>Import</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/workflow`}>Workflow</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/permissions`}>Permissions</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/notifications`}>Notifications</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/audit`}>Audit</NavLink>
      </nav>
      <div className="project-grid">
        {view === "board" && <Board projectKey={key} onOpen={setOpen} />}
        {view === "backlog" && <Backlog projectKey={key} onOpen={setOpen} />}
        {view === "reports" && <Reports projectKey={key} />}
        {view === "import" && <JiraImport projectKey={key} />}
        {view === "workflow" && <WorkflowEditor projectKey={key} />}
        {view === "audit" && <AuditLog projectKey={key} />}
        {view === "releases" && <Releases projectKey={key} />}
        {view === "permissions" && <PermissionScheme projectKey={key} />}
        {view === "notifications" && <TelegramGroups projectKey={key} />}
        <Members projectKey={key} />
      </div>
      {open && (
        <IssuePanel issueKey={open} projectKey={key} onClose={() => setOpen(null)} onOpen={setOpen} extensions={<IssueFocus issueKey={open} />} />
      )}
    </div>
  );
}
