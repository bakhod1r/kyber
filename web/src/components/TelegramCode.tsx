import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../api";

/** "Log in with Telegram code" (KYB-S40). Lives inside the login form, so no nested <form>. */
export function TelegramCode() {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [code, setCode] = useState("");
  const start = useMutation({ mutationFn: api.telegramOtpStart });
  const verify = useMutation({
    mutationFn: () => api.telegramOtpVerify(start.data?.id ?? "", code),
    onSuccess: async () => {
      await qc.fetchQuery({ queryKey: ["me"], queryFn: api.me });
      navigate("/");
    },
  });

  if (!start.data) {
    return (
      <>
        <button type="button" className="ghost social-telegram-code" disabled={start.isPending} onClick={() => start.mutate()}>
          Log in with Telegram code
        </button>
        {start.isError && <p role="alert" className="error">{start.error.message}</p>}
      </>
    );
  }
  const submit = () => code.trim().length === 6 && verify.mutate();
  return (
    <div className="otp" role="group" aria-label="Telegram code">
      <ol>
        <li>
          <a href={start.data.link} target="_blank" rel="noopener noreferrer">
            Open the Kyber bot in Telegram
          </a>{" "}
          and press <b>Start</b>.
        </li>
        <li>Type the 6-digit code it sends you.</li>
      </ol>
      <label>
        Code from Telegram
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
        Verify code
      </button>
    </div>
  );
}
