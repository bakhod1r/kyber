import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { identityUseCases, type TelegramUser } from "../application";
import { TelegramCode } from "./TelegramCode";
import { msg, type MessageKey, useT } from "../../../shared/i18n/i18n";

declare global {
  interface Window {
    kyberTelegramAuth?: (user: TelegramUser) => void;
  }
}

const ERRORS: Record<string, MessageKey> = {
  google_failed: msg("Google sign-in did not complete. Please try again."),
  google_email_taken:
    msg("This email already has a Kyber account, and Google did not confirm it. Log in with your password instead."),
  oidc_failed: msg("Single sign-on did not complete. Please try again."),
  saml_failed: msg("Single sign-on did not complete. Please try again."),
  saml_email_taken: msg("This email already has a Kyber account. Log in with your password instead."),
  oidc_email_taken: msg("This email already has a Kyber account. Log in with your password instead."),
  sso_failed: msg("Single sign-on did not complete. Please try again."),
  sso_email_taken: msg("This email already has a Kyber account. Log in with your password instead."),
  github_failed: msg("GitHub sign-in did not complete. Please try again."),
  github_email_taken:
    msg("This email already has a Kyber account, and GitHub did not confirm it. Log in with your password instead."),
};

/** "Continue with Google", single sign-on, GitHub and the Telegram Login Widget, shown only when configured (KYB-S39, F6). */
export function SocialSignIn() {
  const t = useT();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const providers = useQuery({ queryKey: ["auth-providers"], queryFn: identityUseCases.authProviders, retry: false });
  const widget = useRef<HTMLDivElement>(null);
  const telegram = useMutation({
    mutationFn: identityUseCases.telegramLogin,
    onSuccess: async () => {
      await qc.fetchQuery({ queryKey: ["me"], queryFn: identityUseCases.me });
      navigate("/");
    },
  });
  const bot = providers.data?.telegram_bot ?? null;

  useEffect(() => {
    const host = widget.current;
    if (!bot || !host) return;
    window.kyberTelegramAuth = (user) => telegram.mutate(user);
    const s = document.createElement("script");
    s.async = true;
    s.src = "https://telegram.org/js/telegram-widget.js?22";
    s.dataset.telegramLogin = bot;
    s.dataset.size = "large";
    s.dataset.radius = "8";
    s.dataset.onauth = "kyberTelegramAuth(user)";
    s.dataset.requestAccess = "write";
    host.appendChild(s);
    return () => {
      host.replaceChildren();
      delete window.kyberTelegramAuth;
    };
    // Re-run only when the bot changes (telegram.mutate is stable).
  }, [bot]);

  const known = ERRORS[params.get("error") ?? ""];
  const error = (known && t(known)) ?? (telegram.isError ? telegram.error.message : null);
  const otp = providers.data?.telegram_otp ?? false;
  const sso = providers.data?.oidc ?? null;
  const saml = providers.data?.saml ?? null;
  const github = providers.data?.github ?? false;
  const stored = providers.data?.sso ?? [];
  const any = providers.data?.google || sso || saml || github || bot || otp || stored.length > 0;
  return (
    <>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {any && (
        <div className="social">
          <p className="divider">{t("or continue with")}</p>
          {providers.data?.google && (
            <a className="button ghost social-google" href="/api/v1/auth/google/start">
              <svg aria-hidden="true" width="18" height="18" viewBox="0 0 48 48">
                <path fill="#EA4335" d="M24 9.5c3.5 0 6.6 1.2 9 3.6l6.7-6.7C35.6 2.4 30.2 0 24 0 14.6 0 6.6 5.4 2.6 13.2l7.8 6.1C12.3 13.6 17.7 9.5 24 9.5z" />
                <path fill="#4285F4" d="M46.5 24.5c0-1.6-.1-3.1-.4-4.5H24v9h12.7c-.6 3-2.3 5.5-4.8 7.2l7.6 5.9c4.4-4.1 7-10.1 7-17.6z" />
                <path fill="#FBBC05" d="M10.4 28.7c-.5-1.4-.8-3-.8-4.7s.3-3.2.8-4.7l-7.8-6.1C1 16.5 0 20.1 0 24s1 7.5 2.6 10.8l7.8-6.1z" />
                <path fill="#34A853" d="M24 48c6.5 0 11.9-2.1 15.9-5.8l-7.6-5.9c-2.1 1.4-4.9 2.3-8.3 2.3-6.3 0-11.7-4.1-13.6-9.8l-7.8 6.1C6.6 42.6 14.6 48 24 48z" />
              </svg>
              {t("Continue with Google")}
            </a>
          )}
          {sso && (
            <a className="button ghost social-sso" href="/api/v1/auth/oidc/start">
              <svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <rect x="5" y="11" width="14" height="10" rx="2" />
                <path d="M8 11V7a4 4 0 0 1 8 0v4" />
              </svg>
              {t("Continue with {name}", { name: sso })}
            </a>
          )}
          {stored.map((c) => (
            <a key={c.id} className="button ghost social-sso" href={`/api/v1/auth/sso/${encodeURIComponent(c.id)}/start`}>
              <svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <rect x="5" y="11" width="14" height="10" rx="2" />
                <path d="M8 11V7a4 4 0 0 1 8 0v4" />
              </svg>
              {t("Continue with {name}", { name: c.name })}
            </a>
          ))}
          {saml && (
            <a className="button ghost social-sso" href="/api/v1/auth/saml/start">
              <svg aria-hidden="true" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <rect x="5" y="11" width="14" height="10" rx="2" />
                <path d="M8 11V7a4 4 0 0 1 8 0v4" />
              </svg>
              {t("Continue with {name}", { name: saml })}
            </a>
          )}
          {github && (
            <a className="button ghost social-github" href="/api/v1/auth/github/start">
              <svg aria-hidden="true" width="18" height="18" viewBox="0 0 16 16" fill="currentColor">
                <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0 0 16 8c0-4.42-3.58-8-8-8z" />
              </svg>
              {t("Continue with GitHub")}
            </a>
          )}
          {bot && <div ref={widget} className="social-telegram" />}
          {otp && <TelegramCode />}
        </div>
      )}
    </>
  );
}
