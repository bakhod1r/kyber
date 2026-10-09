import { useMutation, useQueryClient } from "@tanstack/react-query";
import { type ChangeEvent, useState } from "react";
import { Link } from "react-router-dom";
import { type ImportReport, STATUSES, api } from "../api";

const MAX_BYTES = 10 << 20;
const plural = (n: number) => `${n} issue${n === 1 ? "" : "s"}`;
const statusLabel = (id: string) => STATUSES.find((s) => s.id === id)?.label ?? id;

/** Jira CSV import (KYB-S29): pick a file, preview the mapping, then import. */
export function JiraImport({ projectKey }: { projectKey: string }) {
  const qc = useQueryClient();
  const [csv, setCsv] = useState<string | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const run = useMutation({
    mutationFn: (dryRun: boolean) => api.importJira(projectKey, csv ?? "", dryRun),
    onSuccess: (r) => {
      if (!r.dry_run) return qc.invalidateQueries({ queryKey: ["issues", projectKey] });
    },
  });
  const report: ImportReport | undefined = run.data;

  async function onFile(e: ChangeEvent<HTMLInputElement>) {
    run.reset();
    const file = e.target.files?.[0];
    setCsv(null);
    setFileError(null);
    if (!file) return;
    if (file.size > MAX_BYTES) {
      setFileError("The file is larger than 10 MB.");
      return;
    }
    setCsv(await file.text());
  }

  const error = fileError ?? (run.error ? run.error.message : null);
  return (
    <section className="panel import" aria-labelledby="import-title">
      <h2 id="import-title">Import from Jira</h2>
      <p className="hint">
        In Jira, open the issue search, choose <b>Export → CSV (all fields)</b>, then upload the file here. Nothing changes until you
        confirm the preview; importing the same file again skips issues already imported.
      </p>
      <label>
        Jira CSV export
        <input type="file" accept=".csv,text/csv" onChange={onFile} />
      </label>
      <button type="button" disabled={!csv || run.isPending} onClick={() => run.mutate(true)}>
        Preview
      </button>
      {error && <p role="alert" className="error">{error}</p>}
      {report && !report.dry_run && (
        <p role="status">Imported {plural(report.items.length)}.</p>
      )}
      {report && <ReportView report={report} projectKey={projectKey} />}
      {report?.dry_run && report.items.length > 0 && (
        <button type="button" disabled={run.isPending} onClick={() => run.mutate(false)}>
          Import {plural(report.items.length)}
        </button>
      )}
    </section>
  );
}

function ReportView({ report, projectKey }: { report: ImportReport; projectKey: string }) {
  return (
    <>
      {report.skipped.length > 0 && <p>{report.skipped.length} already imported (skipped)</p>}
      {report.errors.length > 0 && (
        <ul aria-label="Rows with errors" className="errors">
          {report.errors.map((e) => (
            <li key={`${e.line}-${e.external_key}`}>
              Line {e.line} · {e.external_key || "?"}: {e.message}
            </li>
          ))}
        </ul>
      )}
      {report.items.length > 0 && (
        <table aria-label={report.dry_run ? "Issues to import" : "Imported issues"}>
          <thead>
            <tr>
              <th scope="col">Jira</th>
              {!report.dry_run && <th scope="col">Kyber</th>}
              <th scope="col">Summary</th>
              <th scope="col">Type</th>
              <th scope="col">Status</th>
              <th scope="col">Priority</th>
              <th scope="col">Points</th>
              <th scope="col">Notes</th>
            </tr>
          </thead>
          <tbody>
            {report.items.map((it) => (
              <tr key={it.external_key}>
                <td>{it.external_key}</td>
                {!report.dry_run && (
                  <td>
                    {it.issue_key && (
                      <Link to={`/projects/${encodeURIComponent(projectKey)}?issue=${encodeURIComponent(it.issue_key)}`}>{it.issue_key}</Link>
                    )}
                  </td>
                )}
                <td>{it.title}</td>
                <td>{it.type}</td>
                <td>{statusLabel(it.status)}</td>
                <td>{it.priority}</td>
                <td>{it.points ?? "—"}</td>
                <td>{it.warnings.join("; ")}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  );
}
