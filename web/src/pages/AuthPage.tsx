import { useMutation, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api } from "../api";
import { SocialSignIn } from "../components/SocialSignIn";

export function AuthPage({ mode }: { mode: "login" | "signup" }) {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const signup = mode === "signup";

  const submit = useMutation({
    mutationFn: async () => {
      if (signup) await api.signup(email, name, password);
      await api.login(email, password);
    },
    onSuccess: async () => {
      // Replace any cached "anonymous" result before routing, or RequireUser
      // would redirect straight back to /login.
      await qc.fetchQuery({ queryKey: ["me"], queryFn: api.me });
      navigate("/");
    },
  });

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    submit.mutate();
  }

  return (
    <main className="auth">
      <form className="panel" onSubmit={onSubmit}>
        <img src="/logo.svg" alt="" width={56} height={56} className="auth-logo" />
        <h1>{signup ? "Create your Kyber account" : "Log in to Kyber"}</h1>
        {signup && (
          <label>
            Name
            <input value={name} onChange={(e) => setName(e.target.value)} required autoComplete="name" />
          </label>
        )}
        <label>
          Email
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required autoComplete="email" />
        </label>
        <label>
          Password
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
          {signup ? "Create account" : "Log in"}
        </button>
        <SocialSignIn />
        <p className="muted">
          {signup ? (
            <>Already have an account? <Link to="/login">Log in</Link></>
          ) : (
            <>New to Kyber? <Link to="/signup">Create an account</Link></>
          )}
        </p>
      </form>
    </main>
  );
}
