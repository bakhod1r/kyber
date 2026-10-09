import { useQuery } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { ApiError, api } from "./api";
import { Layout } from "./components/Layout";
import { AuthPage } from "./pages/AuthPage";
import { ProjectPage } from "./pages/ProjectPage";
import { ProjectsPage } from "./pages/ProjectsPage";

export const ME = ["me"];

function useMe() {
  return useQuery({
    queryKey: ME,
    queryFn: () => api.me().catch((e: unknown) => (e instanceof ApiError && e.status === 401 ? null : Promise.reject(e))),
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

export function AppRoutes() {
  return (
    <Routes>
      <Route path="/login" element={<AuthPage mode="login" />} />
      <Route path="/signup" element={<AuthPage mode="signup" />} />
      <Route path="/" element={<RequireUser><ProjectsPage /></RequireUser>} />
      <Route path="/projects/:key" element={<RequireUser><ProjectPage /></RequireUser>} />
      <Route path="/projects/:key/backlog" element={<RequireUser><ProjectPage view="backlog" /></RequireUser>} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
