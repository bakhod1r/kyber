import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { type CustomField, type Issue, issueUseCases } from "../application";

/** IssueDetails: the project's custom fields on an issue, each with an input for its type. */
export function IssueDetails({ issue, projectKey }: { issue: Issue; projectKey: string }) {
  const qc = useQueryClient();
  const fields = useQuery({ queryKey: ["custom-fields", projectKey], queryFn: () => issueUseCases.customFields(projectKey) });
  const [values, setValues] = useState<Record<string, string>>(issue.custom_fields ?? {});
  const [notice, setNotice] = useState<{ kind: "ok" | "error"; text: string } | null>(null);
  const save = useMutation({
    mutationFn: () => {
      const changed: Record<string, string> = {};
      for (const f of fields.data ?? []) {
        const v = values[f.id] ?? "";
        if (v !== (issue.custom_fields?.[f.id] ?? "")) changed[f.id] = v;
      }
      return issueUseCases.editIssue(issue.key, { version: issue.version, custom_fields: changed });
    },
    onSuccess: (updated) => {
      qc.setQueryData(["issue", issue.key], updated);
      setNotice({ kind: "ok", text: "Details saved" });
    },
    onError: (err: Error) => setNotice({ kind: "error", text: err.message }),
  });
  if (!fields.data || fields.data.length === 0) return null;
  const set = (f: CustomField, v: string) => setValues({ ...values, [f.id]: v });
  return (
    <section className="issue-details" aria-label="Details">
      <h3>Details</h3>
      {fields.data.map((f) => (
        <label key={f.id}>
          {f.name}
          {f.type === "select" ? (
            <select value={values[f.id] ?? ""} onChange={(e) => set(f, e.target.value)}>
              <option value="">None</option>
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
          Save details
        </button>
      </div>
    </section>
  );
}
