import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { deadlineText, GOAL_STATUSES, type Goal, type GoalStatus, goalUseCases } from "../application";

const today = () => new Date().toISOString().slice(0, 10);

/** Goals: the outcomes a project commits to, with progress measured by their JQL (Jira's Goals). */
export function Goals({ projectKey }: { projectKey: string }) {
  const qc = useQueryClient();
  const queryKey = ["goals", projectKey];
  const goals = useQuery({ queryKey, queryFn: () => goalUseCases.goals(projectKey) });
  const [name, setName] = useState("");
  const [target, setTarget] = useState("");
  const [jql, setJql] = useState("");
  const [error, setError] = useState("");
  const refresh = () => qc.invalidateQueries({ queryKey });
  const fail = (err: Error) => setError(err.message);
  const create = useMutation({
    mutationFn: () => goalUseCases.createGoal(projectKey, { name, target_date: target, jql }),
    onSuccess: () => {
      setName("");
      setTarget("");
      setJql("");
      setError("");
      return refresh();
    },
    onError: fail,
  });
  const report = useMutation({
    mutationFn: ({ goal, status }: { goal: Goal; status: GoalStatus }) => goalUseCases.report(goal, status),
    onSuccess: () => {
      setError("");
      return refresh();
    },
    onError: fail,
  });
  const remove = useMutation({ mutationFn: (g: Goal) => goalUseCases.deleteGoal(g.id), onSuccess: refresh, onError: fail });
  const submit = (e: FormEvent) => {
    e.preventDefault();
    create.mutate();
  };
  return (
    <section className="goals" aria-label="Goals">
      <h2>Goals</h2>
      <form className="row wrap" onSubmit={submit}>
        <label>
          Goal name
          <input value={name} onChange={(e) => setName(e.target.value)} required maxLength={120} />
        </label>
        <label>
          Target date
          <input type="date" value={target} onChange={(e) => setTarget(e.target.value)} />
        </label>
        <label>
          Issues (JQL)
          <input value={jql} onChange={(e) => setJql(e.target.value)} placeholder="type = bug AND fixVersion = 1.0" />
        </label>
        <button type="submit" disabled={create.isPending}>
          Add goal
        </button>
      </form>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {goals.isError && (
        <p role="alert" className="error">
          {(goals.error as Error).message}
        </p>
      )}
      {goals.data?.length === 0 && <p className="muted">No goals yet. Add one to track what this project is working towards.</p>}
      <ul className="goal-list">
        {goals.data?.map((g) => (
          <li key={g.id} className={`goal status-${g.status}`}>
            <div className="row between">
              <strong>{g.name}</strong>
              <select aria-label={`Status of ${g.name}`} value={g.status} onChange={(e) => report.mutate({ goal: g, status: e.target.value as GoalStatus })}>
                {GOAL_STATUSES.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.label}
                  </option>
                ))}
              </select>
            </div>
            {g.jql ? (
              <>
                <div className="goal-progress" role="progressbar" aria-label={`Progress of ${g.name}`} aria-valuemin={0} aria-valuemax={100} aria-valuenow={g.percent}>
                  <div style={{ width: `${g.percent}%` }} />
                </div>
                <p className="muted">
                  {g.done} of {g.total} issues done ({g.percent}%) · <code>{g.jql}</code>
                </p>
              </>
            ) : (
              <p className="muted">No issues linked.</p>
            )}
            <div className="row between">
              <span className="muted">{deadlineText(g, today())}</span>
              <button type="button" className="ghost" onClick={() => remove.mutate(g)}>
                Delete goal {g.name}
              </button>
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
}
