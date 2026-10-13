// form: domain model — a project's intake forms; a submission becomes an issue (Jira's Forms).
export type FormIssueType = "story" | "task" | "bug";
export type Question = { id?: string; label: string; required: boolean; long: boolean };
export type Form = {
  id: string;
  project_key: string;
  name: string;
  intro: string;
  issue_type: FormIssueType;
  questions: Question[];
  version: number;
};
export type FormInput = { name: string; intro: string; issue_type: FormIssueType; questions: Question[] };
export type Submission = { summary: string; answers: Record<string, string> };

export const FORM_ISSUE_TYPES: { id: FormIssueType; label: string }[] = [
  { id: "task", label: "Task" },
  { id: "bug", label: "Bug" },
  { id: "story", label: "Story" },
];

export const blankQuestion = (): Question => ({ label: "", required: false, long: false });

/** The labels of required questions still unanswered (checked again by the server). */
export function missingAnswers(form: Pick<Form, "questions">, answers: Record<string, string>): string[] {
  return form.questions.filter((q) => q.required && !(answers[q.id ?? ""] ?? "").trim()).map((q) => q.label);
}
