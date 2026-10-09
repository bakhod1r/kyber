import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { Link, useNavigate } from "react-router-dom";
import { type User, api } from "../api";

export function Layout({ user, children }: { user: User; children: ReactNode }) {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const logout = useMutation({
    mutationFn: api.logout,
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
          Kyber
        </Link>
        <span className="spacer" />
        <span className="user">{user.name}</span>
        <button className="ghost" onClick={() => logout.mutate()}>
          Log out
        </button>
      </header>
      <main className="page">{children}</main>
    </>
  );
}
