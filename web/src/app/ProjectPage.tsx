// ProjectPage composes the project views of every bounded context (app shell / composition root).
import { useQuery } from "@tanstack/react-query";
import { NavLink, useParams, useSearchParams } from "react-router-dom";
import { projectUseCases } from "../contexts/project/application";
import { identityUseCases } from "../contexts/identity/application";
import { Members } from "../contexts/project/ui/Members";
import { ArchiveControl } from "../contexts/project/ui/ArchiveControl";
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
import { Announcements } from "../contexts/announcement/ui/Announcements";
import { Docs } from "../contexts/docs/ui/Docs";
import { Forms } from "../contexts/form/ui/Forms";
import { Goals } from "../contexts/goal/ui/Goals";
import { Webhooks } from "../contexts/webhook/ui/Webhooks";
import { Development } from "../contexts/webhook/ui/Development";
import { Timeline } from "../contexts/issue/ui/Timeline";
import { ProjectSummary } from "../contexts/insights/ui/ProjectSummary";
import { IssueList } from "../contexts/search/ui/IssueList";
import { type QuickFilterState, QuickFilters } from "../contexts/search/ui/QuickFilters";
import { useState } from "react";

/** Docs with who I am and the project's people, so paragraph comments show names (S86). */
function ProjectDocs({ projectKey }: { projectKey: string }) {
  const me = useQuery({ queryKey: ["me"], queryFn: identityUseCases.me });
  const members = useQuery({ queryKey: ["members", projectKey], queryFn: () => projectUseCases.members(projectKey) });
  return <Docs projectKey={projectKey} me={me.data?.id ?? ""} names={new Map((members.data ?? []).map((m) => [m.user_id, m.name || m.email]))} />;
}
import { useT } from "../shared/i18n/i18n";

export function ProjectPage({ view = "board" }: { view?: "summary" | "timeline" | "list" | "goals" | "docs" | "forms" | "announcements" | "board" | "backlog" | "reports" | "import" | "workflow" | "audit" | "notifications" | "permissions" | "releases" | "board-settings" | "fields" | "webhooks" }) {
  const t = useT();
  const key = useParams<{ key: string }>().key ?? "";
  const projects = useQuery({ queryKey: ["projects"], queryFn: projectUseCases.projects });
  const me = useQuery({ queryKey: ["me"], queryFn: identityUseCases.me });
  const project = projects.data?.find((p) => p.key === key);
  const members = useQuery({ queryKey: ["members", key], queryFn: () => projectUseCases.members(key) });
  const admin = members.data?.some((m) => m.user_id === me.data?.id && m.role === "admin") ?? false;
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
      <div className="project-head">
        <h1>
          <span className="key">{key}</span> {project?.name ?? ""}
        </h1>
        <ArchiveControl projectKey={key} admin={admin} />
      </div>
      <nav className="tabs" aria-label={t("Project views")}>
        <NavLink to={`/projects/${encodeURIComponent(key)}/summary`}>{t("Summary")}</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/timeline`}>{t("Timeline")}</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/backlog`}>{t("Backlog")}</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}`} end>
          {t("Active sprints")}
        </NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/reports`}>{t("Reports")}</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/list`}>{t("List")}</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/releases`}>{t("Releases")}</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/goals`}>{t("Goals")}</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/docs`}>{t("Docs")}</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/forms`}>{t("Forms")}</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/announcements`}>{t("Announcements")}</NavLink>
      </nav>
      <nav className="tabs settings-tabs" aria-label={t("Project settings")}>
        <NavLink to={`/projects/${encodeURIComponent(key)}/workflow`}>{t("Workflow")}</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/board-settings`}>{t("Board settings")}</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/fields`}>{t("Fields")}</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/permissions`}>{t("Permissions")}</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/notifications`}>{t("Notifications")}</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/webhooks`}>{t("Webhooks")}</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/import`}>{t("Import")}</NavLink>
        <NavLink to={`/projects/${encodeURIComponent(key)}/audit`}>{t("Audit")}</NavLink>
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
        {view === "docs" && <ProjectDocs projectKey={key} />}
        {view === "announcements" && <Announcements projectKey={key} />}
        {view === "forms" && <Forms projectKey={key} onOpen={setOpen} />}
        {view === "permissions" && <PermissionScheme projectKey={key} />}
        {view === "notifications" && <TelegramGroups projectKey={key} />}
        {view === "webhooks" && <Webhooks projectKey={key} />}
        <Members projectKey={key} />
      </div>
      {open && (
        <IssuePanel issueKey={open} projectKey={key} me={me.data?.id} onClose={() => setOpen(null)} onOpen={setOpen} extensions={
            <>
              <Development issueKey={open} />
              <IssueFocus issueKey={open} />
            </>
          } />
      )}
    </div>
  );
}
