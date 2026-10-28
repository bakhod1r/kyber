import { useMutation, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { issueUseCases, ISSUE_TYPES, type IssueType } from "../../issue/application";
import { useT } from "../../../shared/i18n/i18n";

// A sub-task needs a parent, which a one-line form cannot give, so the server would always reject it;
// sub-tasks are added from their parent issue instead.
const QUICK_ADD_TYPES = ISSUE_TYPES.filter((t) => t !== "subtask");

// QuickAdd creates an issue; during an active sprint it joins that sprint (as in Jira),
// otherwise it would land in the backlog and vanish from the sprint board.
export function QuickAdd({ projectKey, sprintId }: { projectKey: string; sprintId?: string }) {
  const t = useT();
  const qc = useQueryClient();
  const [title, setTitle] = useState("");
  const [type, setType] = useState<IssueType>("task");
  const create = useMutation({
    mutationFn: () => issueUseCases.createIssueIn(projectKey, title, type, sprintId),
    onSuccess: () => {
      setTitle("");
      return qc.invalidateQueries({ queryKey: ["issues", projectKey] });
    },
  });

  function submit(e: FormEvent) {
    e.preventDefault();
    if (title.trim()) create.mutate();
  }

  return (
    <form className="quick-add" onSubmit={submit}>
      <input aria-label={t("New issue title")} placeholder={t("What needs to be done?")} value={title} onChange={(e) => setTitle(e.target.value)} />
      <select aria-label={t("Type")} value={type} onChange={(e) => setType(e.target.value as IssueType)}>
        {QUICK_ADD_TYPES.map((t) => (
          <option key={t} value={t}>
            {t}
          </option>
        ))}
      </select>
      <button type="submit" disabled={create.isPending}>
        {t("Add issue")}
      </button>
      {create.isError && (
        <span role="alert" className="error">
          {create.error.message}
        </span>
      )}
    </form>
  );
}
