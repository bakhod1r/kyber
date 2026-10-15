import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useT } from "../../../shared/i18n/i18n";
import { projectUseCases } from "../application";

/** ArchiveControl: the read-only banner of an archived project; admins archive and restore it (B10). */
export function ArchiveControl({ projectKey, admin }: { projectKey: string; admin: boolean }) {
  const t = useT();
  const qc = useQueryClient();
  const projects = useQuery({ queryKey: ["projects"], queryFn: projectUseCases.projects });
  const project = projects.data?.find((p) => p.key === projectKey);
  const change = useMutation({
    mutationFn: (archive: boolean) => (archive ? projectUseCases.archive(projectKey) : projectUseCases.restore(projectKey)),
    onSuccess: () => qc.invalidateQueries(), // every view's buttons follow the read-only state
  });
  if (!project) return null;
  if (project.archived) {
    return (
      <div role="status" className="archived-banner">
        <span>🗄 {t("This project is archived and read-only.")}</span>
        {admin && (
          <button type="button" className="ghost" disabled={change.isPending} onClick={() => change.mutate(false)}>
            {t("Restore project")}
          </button>
        )}
      </div>
    );
  }
  if (!admin) return null;
  return (
    <button
      type="button"
      className="ghost archive-button"
      disabled={change.isPending}
      onClick={() => window.confirm(t("Archive {name}? It stays readable, but nobody can change it until it is restored.", { name: project.name })) && change.mutate(true)}
    >
      {t("Archive project")}
    </button>
  );
}
