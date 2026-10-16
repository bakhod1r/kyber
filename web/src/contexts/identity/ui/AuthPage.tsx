import { useMutation, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { SocialSignIn } from "./SocialSignIn";
import { identityUseCases, safeNext } from "../application";
import { useT } from "../../../shared/i18n/i18n";

export function AuthPage({ mode }: { mode: "login" | "signup" }) {
  const t = useT();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const next = safeNext(useSearchParams()[0].get("next"));
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const signup = mode === "signup";

  const submit = useMutation({
    mutationFn: async () => {
      if (signup) await identityUseCases.signup(email, name, password);
      await identityUseCases.login(email, password);
    },
    onSuccess: async () => {
      // Replace any cached "anonymous" result before routing, or RequireUser
      // would redirect straight back to /login.
      await qc.fetchQuery({ queryKey: ["me"], queryFn: identityUseCases.me });
      navigate(next);
    },
  });

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    submit.mutate();
  }

  return (
    <main className="auth">
      <form className="panel" onSubmit={onSubmit}>
        <img src={`${import.meta.env.BASE_URL}logo.svg`} alt="" width={56} height={56} className="auth-logo" />
        <h1>{signup ? t("Create your Kyber account") : t("Log in to Kyber")}</h1>
        {signup && (
          <label>
            {t("Name")}
            <input value={name} onChange={(e) => setName(e.target.value)} required autoComplete="name" />
          </label>
        )}
        <label>
          {t("Email")}
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required autoComplete="email" />
        </label>
        <label>
          {t("Password")}
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
            minLength={signup ? 10 : undefined}
            autoComplete={signup ? "new-password" : "current-password"}
          />
        </label>
        {submit.isError && (
          <p role="alert" className="error">
            {submit.error.message}
          </p>
        )}
        <button type="submit" disabled={submit.isPending}>
          {signup ? t("Create account") : t("Log in")}
        </button>
        <SocialSignIn />
        <p className="muted">
          {signup ? (
            <>{t("Already have an account?")} <Link to={next === "/" ? "/login" : `/login?next=${encodeURIComponent(next)}`}>{t("Log in")}</Link></>
          ) : (
            <>{t("New to Kyber?")} <Link to={next === "/" ? "/signup" : `/signup?next=${encodeURIComponent(next)}`}>{t("Create an account")}</Link></>
          )}
        </p>
      </form>
    </main>
  );
}
