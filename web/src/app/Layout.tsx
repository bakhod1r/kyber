import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { Link, NavLink, useNavigate } from "react-router-dom";
import { identityUseCases, type User } from "../contexts/identity/application";
import { FocusTimer } from "../contexts/focus/ui/Focus";
import { NotificationBell } from "../contexts/notify/ui/NotificationBell";
import { useLive } from "../shared/live";
import { adminUseCases } from "../contexts/admin/application";
import { useT } from "../shared/i18n/i18n";
import { LanguagePicker } from "../shared/i18n/LanguagePicker";

export function Layout({ user, children }: { user: User; children: ReactNode }) {
  const t = useT();
  const qc = useQueryClient();
  useLive();
  const admin = useQuery({ queryKey: ["admin", "access"], queryFn: adminUseCases.access, retry: false });
  const navigate = useNavigate();
  const logout = useMutation({
    mutationFn: identityUseCases.logout,
    onSettled: () => {
      qc.clear();
      navigate("/login");
    },
  });
  return (
    <>
      <header className="topbar">
        <Link to="/" className="brand">
          <img src="/logo.svg" alt="" width={28} height={28} />
          {t("Kyber")}
        </Link>
        <nav className="topnav" aria-label={t("Main navigation")}>
          <NavLink to="/search" className="nav-link">
            {t("Search")}
          </NavLink>
          <NavLink to="/chat" className="nav-link">
            {t("Chat")}
          </NavLink>
          <NavLink to="/meetings" className="nav-link">
            {t("Time table")}
          </NavLink>
          {admin.data && (
            <NavLink to="/admin" className="nav-link">
              {t("Admin")}
            </NavLink>
          )}
        </nav>
        <span className="spacer" />
        <FocusTimer />
        <NotificationBell />
        <LanguagePicker />
        <Link to="/settings" className="user" title={t("Notification settings")}>
          {user.name}
        </Link>
        <button className="ghost logout" aria-label={t("Log out")} title={t("Log out")} onClick={() => logout.mutate()}>
          <span aria-hidden="true" className="logout-icon">⎋</span>
          <span className="logout-text">{t("Log out")}</span>
        </button>
      </header>
      <main className="page">{children}</main>
    </>
  );
}
