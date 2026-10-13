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
import { BoardSettings } from "../contexts/agile/ui/BoardSettings";
import { CustomFieldsAdmin } from "../contexts/issue/ui/CustomFieldsAdmin";
import { Releases } from "../contexts/agile/ui/Releases";
import { PermissionScheme } from "../contexts/access/ui/PermissionScheme";
import { TelegramGroups } from "../contexts/notify/ui/TelegramGroups";
import { WorkflowEditor } from "../contexts/issue/ui/WorkflowEditor";
import { JiraImport } from "../contexts/importer/ui/ImportPage";
import { Reports } from "../contexts/insights/ui/ReportsPage";
import { Goals } from "../contexts/goal/ui/Goals";
import { Timeline } from "../contexts/issue/ui/Timeline";
import { ProjectSummary } from "../contexts/insights/ui/ProjectSummary";
import { IssueList } from "../contexts/search/ui/IssueList";
import { type QuickFilterState, QuickFilters } from "../contexts/search/ui/QuickFilters";
import { useState } from "react";

export function ProjectPage({ view = "board" }: { view?: "summary" | "timeline" | "list" | "goals" | "board" | "backlog" | "reports" | "import" | "workflow" | "audit" | "notifications" | "permissions" | "releases" | "board-settings" | "fields" }) {
  const key = useParams<{ key: string }>().key ?? "";
  const projects = useQuery({ queryKey: ["projects"], queryFn: projectUseCases.projects });
  const project = projects.data?.find((p) => p.key === key);
  // The open issue lives in the URL (?issue=KEY-1), so notifications and links can deep-link to it.
  const [params, setParams] = useSearchParams();
  const open = params.get("issue");
  const [filters, setFilters] = useState<QuickFilterState>({ quick: [], saved: "" });
  const [visible, setVisible] = useState<Set<string> | null>(null);
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
        <NavLink to={`/projects/${encodeURIComponent(key)}/summary`}>Summary</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/timeline`}>Timeline</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/backlog`}>Backlog</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}`} end>
          Active sprints
        </NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/reports`}>Reports</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/list`}>List</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/releases`}>Releases</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/goals`}>Goals</NavLink>
      </nav>
      <nav className="tabs settings-tabs" aria-label="Project settings">
        <NavLink to={`/projects/${encodeURIComponent(key)}/workflow`}>Workflow</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/board-settings`}>Board settings</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/fields`}>Fields</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/permissions`}>Permissions</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/notifications`}>Notifications</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/import`}>Import</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/audit`}>Audit</NavLink>
      </nav>
      <div className="project-grid">
        {view === "summary" && <ProjectSummary projectKey={key} />}
        {view === "timeline" && <Timeline projectKey={key} onOpen={setOpen} />}
        {view === "list" && <IssueList projectKey={key} onOpen={setOpen} />}
        {view === "board" && (
          <div>
            <QuickFilters projectKey={key} value={filters} onChange={setFilters} onKeys={setVisible} />
            <Board projectKey={key} onOpen={setOpen} visibleKeys={visible} />
          </div>
        )}
        {view === "backlog" && <Backlog projectKey={key} onOpen={setOpen} />}
        {view === "reports" && <Reports projectKey={key} />}
        {view === "import" && <JiraImport projectKey={key} />}
        {view === "workflow" && <WorkflowEditor projectKey={key} />}
        {view === "audit" && <AuditLog projectKey={key} />}
        {view === "fields" && <CustomFieldsAdmin projectKey={key} />}
        {view === "board-settings" && <BoardSettings projectKey={key} />}
        {view === "releases" && <Releases projectKey={key} />}
        {view === "goals" && <Goals projectKey={key} />}
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
