import { useQuery } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { ApiError } from "../shared/http";
import { Layout } from "./Layout";
import { AuthPage } from "../contexts/identity/ui/AuthPage";
import { LandingPage } from "./LandingPage";
import { MeetingsPage } from "../contexts/calendar/ui/MeetingsPage";
import { ChatPage } from "../contexts/chat/ui/ChatPage";
import { ProjectPage } from "./ProjectPage";
import { SettingsPage } from "../contexts/notify/ui/SettingsPage";
import { ProjectsPage } from "../contexts/project/ui/ProjectsPage";
import { identityUseCases } from "../contexts/identity/application";
import { SearchPage } from "../contexts/search/ui/SearchPage";

export const ME = ["me"];

function useMe() {
  return useQuery({
    queryKey: ME,
    queryFn: () => identityUseCases.me().catch((e: unknown) => (e instanceof ApiError && e.status === 401 ? null : Promise.reject(e))),
    staleTime: 60_000,
  });
}

function RequireUser({ children }: { children: ReactNode }) {
  const me = useMe();
  if (me.isPending) return <p className="muted center">Loading…</p>;
  if (me.isError) return <p role="alert" className="error center">{me.error.message}</p>;
  if (!me.data) return <Navigate to="/login" replace />;
  return <Layout user={me.data}>{children}</Layout>;
}

/** "/" is the landing page for visitors and the project list for signed-in users. */
function Home() {
  const me = useMe();
  if (me.isPending) return <p className="muted center">Loading…</p>;
  if (me.isError) return <p role="alert" className="error center">{me.error.message}</p>;
  if (!me.data) return <LandingPage />;
  return (
    <Layout user={me.data}>
      <ProjectsPage />
    </Layout>
  );
}

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<AuthPage mode="login" />} />
      <Route path="/signup" element={<AuthPage mode="signup" />} />
      <Route path="/" element={<Home />} />
      <Route path="/projects/:key" element={<RequireUser><ProjectPage /></RequireUser>} />
      <Route path="/projects/:key/backlog" element={<RequireUser><ProjectPage view="backlog" /></RequireUser>} />
      <Route path="/projects/:key/reports" element={<RequireUser><ProjectPage view="reports" /></RequireUser>} />
      <Route path="/projects/:key/import" element={<RequireUser><ProjectPage view="import" /></RequireUser>} />
      <Route path="/projects/:key/workflow" element={<RequireUser><ProjectPage view="workflow" /></RequireUser>} />
      <Route path="/projects/:key/audit" element={<RequireUser><ProjectPage view="audit" /></RequireUser>} />
      <Route path="/projects/:key/summary" element={<RequireUser><ProjectPage view="summary" /></RequireUser>} />
      <Route path="/projects/:key/list" element={<RequireUser><ProjectPage view="list" /></RequireUser>} />
      <Route path="/projects/:key/fields" element={<RequireUser><ProjectPage view="fields" /></RequireUser>} />
      <Route path="/projects/:key/board-settings" element={<RequireUser><ProjectPage view="board-settings" /></RequireUser>} />
      <Route path="/projects/:key/releases" element={<RequireUser><ProjectPage view="releases" /></RequireUser>} />
      <Route path="/projects/:key/permissions" element={<RequireUser><ProjectPage view="permissions" /></RequireUser>} />
      <Route path="/projects/:key/notifications" element={<RequireUser><ProjectPage view="notifications" /></RequireUser>} />
      <Route path="/chat" element={<RequireUser><ChatPage /></RequireUser>} />
      <Route path="/chat/:id" element={<RequireUser><ChatPage /></RequireUser>} />
      <Route path="/meetings" element={<RequireUser><MeetingsPage /></RequireUser>} />
      <Route path="/search" element={<RequireUser><SearchPage /></RequireUser>} />
      <Route path="/settings" element={<RequireUser><SettingsPage /></RequireUser>} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
