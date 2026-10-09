import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";
import { type TelegramUser, api } from "../api";

declare global {
  interface Window {
    kyberTelegramAuth?: (user: TelegramUser) => void;
  }
}

const ERRORS: Record<string, string> = {
  google_failed: "Google sign-in did not complete. Please try again.",
  google_email_taken:
    "This email already has a Kyber account, and Google did not confirm it. Log in with your password instead.",
};

/** "Continue with Google" and the Telegram Login Widget, shown only when configured (KYB-S39). */
export function SocialSignIn() {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [params] = useSearchParams();
  const providers = useQuery({ queryKey: ["auth-providers"], queryFn: api.authProviders, retry: false });
  const widget = useRef<HTMLDivElement>(null);
  const telegram = useMutation({
    mutationFn: api.telegramLogin,
    onSuccess: async () => {
      await qc.fetchQuery({ queryKey: ["me"], queryFn: api.me });
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

  const error = ERRORS[params.get("error") ?? ""] ?? (telegram.isError ? telegram.error.message : null);
  const any = providers.data?.google || bot;
  return (
    <>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {any && (
        <div className="social">
          <p className="divider">or continue with</p>
          {providers.data?.google && (
            <a className="button ghost social-google" href="/api/v1/auth/google/start">
              <svg aria-hidden="true" width="18" height="18" viewBox="0 0 48 48">
                <path fill="#EA4335" d="M24 9.5c3.5 0 6.6 1.2 9 3.6l6.7-6.7C35.6 2.4 30.2 0 24 0 14.6 0 6.6 5.4 2.6 13.2l7.8 6.1C12.3 13.6 17.7 9.5 24 9.5z" />
                <path fill="#4285F4" d="M46.5 24.5c0-1.6-.1-3.1-.4-4.5H24v9h12.7c-.6 3-2.3 5.5-4.8 7.2l7.6 5.9c4.4-4.1 7-10.1 7-17.6z" />
                <path fill="#FBBC05" d="M10.4 28.7c-.5-1.4-.8-3-.8-4.7s.3-3.2.8-4.7l-7.8-6.1C1 16.5 0 20.1 0 24s1 7.5 2.6 10.8l7.8-6.1z" />
                <path fill="#34A853" d="M24 48c6.5 0 11.9-2.1 15.9-5.8l-7.6-5.9c-2.1 1.4-4.9 2.3-8.3 2.3-6.3 0-11.7-4.1-13.6-9.8l-7.8 6.1C6.6 42.6 14.6 48 24 48z" />
              </svg>
              Continue with Google
            </a>
          )}
          {bot && <div ref={widget} className="social-telegram" />}
        </div>
      )}
    </>
  );
}
