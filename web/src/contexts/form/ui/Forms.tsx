import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { blankQuestion, type Form, FORM_ISSUE_TYPES, type FormInput, type FormIssueType, formUseCases, missingAnswers, type Question } from "../application";
import { useT } from "../../../shared/i18n/i18n";

type Mode = { kind: "list" } | { kind: "design"; form: Form | null } | { kind: "fill"; form: Form };

/** Forms: intake forms of the project; filling one in creates an issue (Jira's Forms). */
export function Forms({ projectKey, onOpen }: { projectKey: string; onOpen: (issueKey: string) => void }) {
  const t = useT();
  const qc = useQueryClient();
  const listKey = ["forms", projectKey];
  const forms = useQuery({ queryKey: listKey, queryFn: () => formUseCases.forms(projectKey) });
  const [mode, setMode] = useState<Mode>({ kind: "list" });
  const [error, setError] = useState("");
  const [created, setCreated] = useState("");
  const back = () => {
    setError("");
    setMode({ kind: "list" });
    return qc.invalidateQueries({ queryKey: listKey });
  };
  const remove = useMutation({ mutationFn: (f: Form) => formUseCases.deleteForm(f.id), onSuccess: back, onError: (e: Error) => setError(e.message) });

  return (
    <section className="forms" aria-label={t("Forms")}>
      <h2>{t("Forms")}</h2>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {created && (
        <p role="status">
          {t("Created")}{" "}
          <button type="button" className="ghost" onClick={() => onOpen(created)}>
            {created}
          </button>
        </p>
      )}
      {mode.kind === "design" && <FormDesigner key={mode.form?.id ?? "new"} projectKey={projectKey} form={mode.form} onDone={back} onCancel={() => setMode({ kind: "list" })} />}
      {mode.kind === "fill" && (
        <FormFiller
          key={mode.form.id}
          form={mode.form}
          onDone={(key) => {
            setCreated(key);
            setMode({ kind: "list" });
            return qc.invalidateQueries({ queryKey: ["issues", projectKey] });
          }}
          onCancel={() => setMode({ kind: "list" })}
        />
      )}
      {mode.kind === "list" && (
        <>
          <button type="button" onClick={() => setMode({ kind: "design", form: null })}>
            {t("New form")}
          </button>
          {forms.isError && (
            <p role="alert" className="error">
              {(forms.error as Error).message}
            </p>
          )}
          {forms.data?.length === 0 && <p className="muted">{t("No forms yet. Project admins can add one so others can request work.")}</p>}
          <ul className="form-list">
            {forms.data?.map((f) => (
              <li key={f.id} className="form-card">
                <strong>{f.name}</strong> <span className="muted">{t("creates a {type}", { type: f.issue_type })}</span>
                {f.intro && <p className="muted">{f.intro}</p>}
                <div className="row">
                  <button type="button" onClick={() => setMode({ kind: "fill", form: f })}>
                    {t("Fill in {name}", { name: f.name })}
                  </button>
                  <button type="button" className="ghost" onClick={() => setMode({ kind: "design", form: f })}>
                    {t("Edit {name}", { name: f.name })}
                  </button>
                  <button type="button" className="ghost" onClick={() => remove.mutate(f)}>
                    {t("Delete {name}", { name: f.name })}
                  </button>
                </div>
              </li>
            ))}
          </ul>
        </>
      )}
    </section>
  );
}

function FormDesigner({ projectKey, form, onDone, onCancel }: { projectKey: string; form: Form | null; onDone: () => void; onCancel: () => void }) {
  const t = useT();
  const [input, setInput] = useState<FormInput>({ name: form?.name ?? "", intro: form?.intro ?? "", issue_type: form?.issue_type ?? "task", questions: form?.questions ?? [] });
  const save = useMutation({
    mutationFn: () => (form ? formUseCases.redesignForm(form.id, form.version, input) : formUseCases.createForm(projectKey, input)),
    onSuccess: onDone,
  });
  const setQuestion = (i: number, q: Partial<Question>) =>
    setInput({ ...input, questions: input.questions.map((old, j) => (j === i ? { ...old, ...q } : old)) });
  const submit = (e: FormEvent) => {
    e.preventDefault();
    save.mutate();
  };
  return (
    <form className="form-designer" aria-label={t("Form design")} onSubmit={submit}>
      <label>
        {t("Form name")}
        <input value={input.name} onChange={(e) => setInput({ ...input, name: e.target.value })} required maxLength={80} />
      </label>
      <label>
        {t("Intro")}
        <textarea rows={2} value={input.intro} onChange={(e) => setInput({ ...input, intro: e.target.value })} maxLength={2000} />
      </label>
      <label>
        {t("Creates")}
        <select value={input.issue_type} onChange={(e) => setInput({ ...input, issue_type: e.target.value as FormIssueType })}>
          {FORM_ISSUE_TYPES.map((t) => (
            <option key={t.id} value={t.id}>
              {t.label}
            </option>
          ))}
        </select>
      </label>
      <fieldset>
        <legend>{t("Questions (a summary is always asked)")}</legend>
        {input.questions.map((q, i) => (
          <div key={i} className="row">
            <input aria-label={t("Question {n}", { n: i + 1 })} value={q.label} onChange={(e) => setQuestion(i, { label: e.target.value })} maxLength={120} />
            <label>
              <input type="checkbox" checked={q.required} onChange={(e) => setQuestion(i, { required: e.target.checked })} /> {t("Required")}
            </label>
            <label>
              <input type="checkbox" checked={q.long} onChange={(e) => setQuestion(i, { long: e.target.checked })} /> {t("Long answer")}
            </label>
            <button type="button" className="ghost" onClick={() => setInput({ ...input, questions: input.questions.filter((_, j) => j !== i) })}>
              {t("Remove question {n}", { n: i + 1 })}
            </button>
          </div>
        ))}
        <button type="button" className="ghost" onClick={() => setInput({ ...input, questions: [...input.questions, blankQuestion()] })}>
          {t("Add question")}
        </button>
      </fieldset>
      {save.isError && (
        <p role="alert" className="error">
          {save.error.message}
        </p>
      )}
      <div className="row">
        <button type="submit" disabled={save.isPending}>
          {t("Save form")}
        </button>
        <button type="button" className="ghost" onClick={onCancel}>
          {t("Cancel")}
        </button>
      </div>
    </form>
  );
}

function FormFiller({ form, onDone, onCancel }: { form: Form; onDone: (issueKey: string) => void; onCancel: () => void }) {
  const t = useT();
  const [summary, setSummary] = useState("");
  const [answers, setAnswers] = useState<Record<string, string>>({});
  const [missing, setMissing] = useState<string[]>([]);
  const send = useMutation({ mutationFn: () => formUseCases.submit(form.id, { summary, answers }), onSuccess: (r) => onDone(r.issue_key) });
  const submit = (e: FormEvent) => {
    e.preventDefault();
    const left = missingAnswers(form, answers);
    setMissing(left);
    if (left.length === 0) send.mutate();
  };
  return (
    <form className="form-filler" aria-label={form.name} onSubmit={submit}>
      <h3>{form.name}</h3>
      {form.intro && <p className="muted">{form.intro}</p>}
      <label>
        {t("Summary")}
        <input value={summary} onChange={(e) => setSummary(e.target.value)} required maxLength={255} />
      </label>
      {form.questions.map((q) => {
        const id = q.id ?? "";
        const set = (v: string) => setAnswers({ ...answers, [id]: v });
        return (
          <label key={id}>
            {q.label}
            {q.required ? " *" : ""}
            {q.long ? <textarea rows={4} value={answers[id] ?? ""} onChange={(e) => set(e.target.value)} /> : <input value={answers[id] ?? ""} onChange={(e) => set(e.target.value)} />}
          </label>
        );
      })}
      {missing.length > 0 && (
        <p role="alert" className="error">
          {t("Please answer: {list}", { list: missing.join(", ") })}
        </p>
      )}
      {send.isError && (
        <p role="alert" className="error">
          {send.error.message}
        </p>
      )}
      <div className="row">
        <button type="submit" disabled={send.isPending}>
          {t("Submit")}
        </button>
        <button type="button" className="ghost" onClick={onCancel}>
          {t("Cancel")}
        </button>
      </div>
    </form>
  );
}
