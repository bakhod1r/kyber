import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { identityUseCases } from "../application";
import { useT } from "../../../shared/i18n/i18n";

/** "Log in with Telegram code" (KYB-S40). Lives inside the login form, so no nested <form>. */
export function TelegramCode() {
  const t = useT();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [code, setCode] = useState("");
  const start = useMutation({ mutationFn: identityUseCases.telegramOtpStart });
  const verify = useMutation({
    mutationFn: () => identityUseCases.telegramOtpVerify(start.data?.id ?? "", code),
    onSuccess: async () => {
      await qc.fetchQuery({ queryKey: ["me"], queryFn: identityUseCases.me });
      navigate("/");
    },
  });

  if (!start.data) {
    return (
      <>
        <button type="button" className="ghost social-telegram-code" disabled={start.isPending} onClick={() => start.mutate()}>
          {t("Log in with Telegram code")}
        </button>
        {start.isError && <p role="alert" className="error">{start.error.message}</p>}
      </>
    );
  }
  const submit = () => code.trim().length === 6 && verify.mutate();
  return (
    <div className="otp" role="group" aria-label={t("Telegram code")}>
      <ol>
        <li>
          <a href={start.data.link} target="_blank" rel="noopener noreferrer">
            {t("Open the Kyber bot in Telegram")}
          </a>{" "}
          {t("and press")} <b>{t("Start")}</b>.
        </li>
        <li>{t("Type the 6-digit code it sends you.")}</li>
      </ol>
      <label>
        {t("Code from Telegram")}
        <input
          value={code}
          onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              submit();
            }
          }}
          inputMode="numeric"
          autoComplete="one-time-code"
          pattern="[0-9]{6}"
          placeholder="123456"
        />
      </label>
      {verify.isError && <p role="alert" className="error">{verify.error.message}</p>}
      <button type="button" disabled={verify.isPending || code.length !== 6} onClick={submit}>
        {t("Verify code")}
      </button>
    </div>
  );
}
