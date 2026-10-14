import { useMutation, useQueryClient } from "@tanstack/react-query";
import { type ChangeEvent, useState } from "react";
import { Link } from "react-router-dom";
import { importerUseCases, type ImportReport } from "../application";
import { STATUSES } from "../../issue/application";
import { useT } from "../../../shared/i18n/i18n";

const MAX_BYTES = 10 << 20;
const statusLabel = (id: string) => STATUSES.find((s) => s.id === id)?.label ?? id;

/** Jira CSV import (KYB-S29): pick a file, preview the mapping, then import. */
export function JiraImport({ projectKey }: { projectKey: string }) {
  const t = useT();
  const qc = useQueryClient();
  const [csv, setCsv] = useState<string | null>(null);
  const [fileError, setFileError] = useState<string | null>(null);
  const run = useMutation({
    mutationFn: (dryRun: boolean) => importerUseCases.importJira(projectKey, csv ?? "", dryRun),
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
      setFileError(t("The file is larger than 10 MB."));
      return;
    }
    setCsv(await file.text());
  }

  const error = fileError ?? (run.error ? run.error.message : null);
  return (
    <section className="panel import" aria-labelledby="import-title">
      <h2 id="import-title">{t("Import & export")}</h2>
      <p>
        <a href={`/api/v1/projects/${encodeURIComponent(projectKey)}/export.csv`} download>
          {t("Export all issues (CSV)")}
        </a>{" "}
        — Jira-compatible; it imports back here or into Jira.
      </p>
      <h3>{t("Import from Jira")}</h3>
      <p className="hint">
        {t("In Jira, open the issue search, choose")} <b>{t("Export → CSV (all fields)")}</b>
        {t(", then upload the file here. Nothing changes until you confirm the preview; importing the same file again skips issues already imported.")}
      </p>
      <label>
        {t("Jira CSV export")}
        <input type="file" accept=".csv,text/csv" onChange={onFile} />
      </label>
      <button type="button" disabled={!csv || run.isPending} onClick={() => run.mutate(true)}>
        {t("Preview")}
      </button>
      {error && <p role="alert" className="error">{error}</p>}
      {report && !report.dry_run && (
        <p role="status">{(report.items.length === 1 ? t("Imported {n} issue.", { n: 1 }) : t("Imported {n} issues.", { n: report.items.length }))}</p>
      )}
      {report && <ReportView report={report} projectKey={projectKey} />}
      {report?.dry_run && report.items.length > 0 && (
        <button type="button" disabled={run.isPending} onClick={() => run.mutate(false)}>
          {(report.items.length === 1 ? t("Import {n} issue", { n: 1 }) : t("Import {n} issues", { n: report.items.length }))}
        </button>
      )}
    </section>
  );
}

function ReportView({ report, projectKey }: { report: ImportReport; projectKey: string }) {
  const t = useT();
  return (
    <>
      {report.skipped.length > 0 && <p>{report.skipped.length} already imported (skipped)</p>}
      {report.errors.length > 0 && (
        <ul aria-label={t("Rows with errors")} className="errors">
          {report.errors.map((e) => (
            <li key={`${e.line}-${e.external_key}`}>
              {t("Line {n}", { n: e.line })} · {e.external_key || "?"}: {e.message}
            </li>
          ))}
        </ul>
      )}
      {report.items.length > 0 && (
        <table aria-label={report.dry_run ? t("Issues to import") : t("Imported issues")}>
          <thead>
            <tr>
              <th scope="col">{t("Jira")}</th>
              {!report.dry_run && <th scope="col">{t("Kyber")}</th>}
              <th scope="col">{t("Summary")}</th>
              <th scope="col">{t("Type")}</th>
              <th scope="col">{t("Status")}</th>
              <th scope="col">{t("Priority")}</th>
              <th scope="col">{t("Points")}</th>
              <th scope="col">{t("Notes")}</th>
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
