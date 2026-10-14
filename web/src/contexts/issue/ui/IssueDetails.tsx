import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { type CustomField, type Issue, type IssuePatch, issueUseCases } from "../application";
import { useT } from "../../../shared/i18n/i18n";

/** IssueDetails: the issue's start and due dates and the project's custom fields, each with an input for its type. */
export function IssueDetails({ issue, projectKey }: { issue: Issue; projectKey: string }) {
  const t = useT();
  const qc = useQueryClient();
  const fields = useQuery({ queryKey: ["custom-fields", projectKey], queryFn: () => issueUseCases.customFields(projectKey) });
  const [values, setValues] = useState<Record<string, string>>(issue.custom_fields ?? {});
  const [start, setStart] = useState(issue.start_date ?? "");
  const [due, setDue] = useState(issue.due_date ?? "");
  const [notice, setNotice] = useState<{ kind: "ok" | "error"; text: string } | null>(null);
  const save = useMutation({
    mutationFn: () => {
      const changed: Record<string, string> = {};
      for (const f of fields.data ?? []) {
        const v = values[f.id] ?? "";
        if (v !== (issue.custom_fields?.[f.id] ?? "")) changed[f.id] = v;
      }
      const patch: IssuePatch = { version: issue.version, custom_fields: changed };
      if (start !== (issue.start_date ?? "")) patch.start_date = start;
      if (due !== (issue.due_date ?? "")) patch.due_date = due;
      return issueUseCases.editIssue(issue.key, patch);
    },
    onSuccess: (updated) => {
      qc.setQueryData(["issue", issue.key], updated);
      void qc.invalidateQueries({ queryKey: ["issues", projectKey] });
      setNotice({ kind: "ok", text: t("Details saved") });
    },
    onError: (err: Error) => setNotice({ kind: "error", text: err.message }),
  });
  const set = (f: CustomField, v: string) => setValues({ ...values, [f.id]: v });
  return (
    <section className="issue-details" aria-label={t("Details")}>
      <h3>{t("Details")}</h3>
      <label>
        {t("Start date")}
        <input type="date" value={start} onChange={(e) => setStart(e.target.value)} />
      </label>
      <label>
        {t("Due date")}
        <input type="date" value={due} onChange={(e) => setDue(e.target.value)} />
      </label>
      {(fields.data ?? []).map((f) => (
        <label key={f.id}>
          {f.name}
          {f.type === "select" ? (
            <select value={values[f.id] ?? ""} onChange={(e) => set(f, e.target.value)}>
              <option value="">{t("None")}</option>
              {f.options.map((o) => (
                <option key={o} value={o}>
                  {o}
                </option>
              ))}
            </select>
          ) : (
            <input
              type={f.type === "number" ? "number" : f.type === "date" ? "date" : "text"}
              step={f.type === "number" ? "any" : undefined}
              value={values[f.id] ?? ""}
              onChange={(e) => set(f, e.target.value)}
            />
          )}
        </label>
      ))}
      <div className="row end">
        {notice && (
          <span role={notice.kind === "error" ? "alert" : "status"} className={notice.kind === "error" ? "error" : "muted"}>
            {notice.text}
          </span>
        )}
        <button type="button" disabled={save.isPending} onClick={() => save.mutate()}>
          {t("Save details")}
        </button>
      </div>
    </section>
  );
}
