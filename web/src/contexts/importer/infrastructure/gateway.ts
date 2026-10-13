// importer: HTTP gateway to the importer API (infrastructure).
import { request, seg } from "../../../shared/http";
import type { ImportReport } from "../domain/model";

export const importerGateway = {
  /** Previews (dryRun) or runs a Jira CSV import; admins only. */
  importJira: (key: string, csv: string, dryRun: boolean) =>
    request<ImportReport>("POST", `/api/v1/projects/${seg(key)}/import/jira?dry_run=${dryRun}`, { csv }),
};
