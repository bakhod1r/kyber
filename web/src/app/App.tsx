import { useQuery } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { ApiError } from "../shared/http";
import { Layout } from "./Layout";
import { AuthPage } from "../contexts/identity/ui/AuthPage";
import { LandingPage } from "./LandingPage";
import { MeetingsPage } from "../contexts/calendar/ui/MeetingsPage";
import { ChatPage, type ConversationSlot } from "../contexts/chat/ui/ChatPage";
import { chatUseCases } from "../contexts/chat/application";
import { ConversationExtras } from "./ConversationExtras";
import { ChatMentions } from "../contexts/notify/ui/ChatMentions";
import { issueFromMessage, runChatCommand } from "./chatCommands";

/** Huddle and whiteboards of the open conversation, composed into the chat. */
const extras: ConversationSlot = (conversationId, me, names) => (
  <ConversationExtras key={conversationId} conversationId={conversationId} me={me} names={names} />
);
import { AdminPage } from "../contexts/admin/ui/AdminPage";
import { Announcements } from "../contexts/announcement/ui/Announcements";
import { ProjectPage } from "./ProjectPage";
import { SettingsPage } from "../contexts/notify/ui/SettingsPage";
import { AccessTokens } from "../contexts/identity/ui/AccessTokens";
import { AppearanceSettings } from "../shared/ui/AppearanceSettings";
import { ProjectsPage } from "../contexts/project/ui/ProjectsPage";
import { identityUseCases } from "../contexts/identity/application";
import { SearchPage } from "../contexts/search/ui/SearchPage";
import { useT } from "../shared/i18n/i18n";

export const ME = ["me"];

/** The time table, with the channels a meeting can happen in (S76). */
function Meetings() {
  const convs = useQuery({ queryKey: ["chat", "conversations"], queryFn: chatUseCases.conversations });
  const channels = (convs.data ?? []).filter((c) => c.kind === "channel" && c.member && !c.archived).map((c) => ({ id: c.id, name: c.name }));
  return <MeetingsPage channels={channels} />;
}

function useMe() {
  return useQuery({
    queryKey: ME,
    queryFn: () => identityUseCases.me().catch((e: unknown) => (e instanceof ApiError && e.status === 401 ? null : Promise.reject(e))),
    staleTime: 60_000,
  });
}

function RequireUser({ children }: { children: ReactNode }) {
  const t = useT();
  const me = useMe();
  if (me.isPending) return <p className="muted center">{t("Loading…")}</p>;
  if (me.isError) return <p role="alert" className="error center">{me.error.message}</p>;
  if (!me.data) return <Navigate to="/login" replace />;
  return <Layout user={me.data}>{children}</Layout>;
}

/** "/" is the landing page for visitors and the project list for signed-in users. */
function Home() {
  const t = useT();
  const me = useMe();
  if (me.isPending) return <p className="muted center">{t("Loading…")}</p>;
  if (me.isError) return <p role="alert" className="error center">{me.error.message}</p>;
  if (!me.data) return <LandingPage />;
  return (
    <Layout user={me.data}>
      <Announcements projectKey={null} />
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
      <Route path="/admin" element={<RequireUser><AdminPage /></RequireUser>} />
      <Route path="/projects/:key" element={<RequireUser><ProjectPage /></RequireUser>} />
      <Route path="/projects/:key/backlog" element={<RequireUser><ProjectPage view="backlog" /></RequireUser>} />
      <Route path="/projects/:key/reports" element={<RequireUser><ProjectPage view="reports" /></RequireUser>} />
      <Route path="/projects/:key/import" element={<RequireUser><ProjectPage view="import" /></RequireUser>} />
      <Route path="/projects/:key/workflow" element={<RequireUser><ProjectPage view="workflow" /></RequireUser>} />
      <Route path="/projects/:key/audit" element={<RequireUser><ProjectPage view="audit" /></RequireUser>} />
      <Route path="/projects/:key/summary" element={<RequireUser><ProjectPage view="summary" /></RequireUser>} />
      <Route path="/projects/:key/timeline" element={<RequireUser><ProjectPage view="timeline" /></RequireUser>} />
      <Route path="/projects/:key/announcements" element={<RequireUser><ProjectPage view="announcements" /></RequireUser>} />
      <Route path="/projects/:key/forms" element={<RequireUser><ProjectPage view="forms" /></RequireUser>} />
      <Route path="/projects/:key/docs" element={<RequireUser><ProjectPage view="docs" /></RequireUser>} />
      <Route path="/projects/:key/goals" element={<RequireUser><ProjectPage view="goals" /></RequireUser>} />
      <Route path="/projects/:key/list" element={<RequireUser><ProjectPage view="list" /></RequireUser>} />
      <Route path="/projects/:key/fields" element={<RequireUser><ProjectPage view="fields" /></RequireUser>} />
      <Route path="/projects/:key/board-settings" element={<RequireUser><ProjectPage view="board-settings" /></RequireUser>} />
      <Route path="/projects/:key/releases" element={<RequireUser><ProjectPage view="releases" /></RequireUser>} />
      <Route path="/projects/:key/permissions" element={<RequireUser><ProjectPage view="permissions" /></RequireUser>} />
      <Route path="/projects/:key/notifications" element={<RequireUser><ProjectPage view="notifications" /></RequireUser>} />
      <Route path="/chat" element={<RequireUser><ChatPage extras={extras} activity={<ChatMentions />} commands={runChatCommand} issueMaker={issueFromMessage} /></RequireUser>} />
      <Route path="/chat/:id" element={<RequireUser><ChatPage extras={extras} activity={<ChatMentions />} commands={runChatCommand} issueMaker={issueFromMessage} /></RequireUser>} />
      <Route path="/meetings" element={<RequireUser><Meetings /></RequireUser>} />
      <Route path="/projects/:key/webhooks" element={<RequireUser><ProjectPage view="webhooks" /></RequireUser>} />
      <Route path="/search" element={<RequireUser><SearchPage /></RequireUser>} />
      <Route path="/settings" element={<RequireUser><SettingsPage /><AppearanceSettings /><AccessTokens /></RequireUser>} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
