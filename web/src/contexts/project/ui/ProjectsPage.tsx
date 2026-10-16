import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { projectUseCases } from "../application";
import { useT } from "../../../shared/i18n/i18n";

export function ProjectsPage() {
  const t = useT();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const projects = useQuery({ queryKey: ["projects"], queryFn: projectUseCases.projects });
  const [key, setKey] = useState("");
  const [name, setName] = useState("");
  const create = useMutation({
    mutationFn: () => projectUseCases.createProject(key, name),
    onSuccess: async (p) => {
      await qc.invalidateQueries({ queryKey: ["projects"] });
      navigate(`/projects/${encodeURIComponent(p.key)}`);
    },
  });

  function onSubmit(e: FormEvent) {
    e.preventDefault();
    create.mutate();
  }

  return (
    <div className="projects">
      <h1>{t("Projects")}</h1>
      {projects.isPending && <p className="muted">{t("Loading…")}</p>}
      {projects.isError && <p role="alert" className="error">{projects.error.message}</p>}
      {projects.data?.length === 0 && <p className="muted">{t("No projects yet — create the first one below.")}</p>}
      <ul className="project-list">
        {projects.data?.map((p) => (
          <li key={p.id}>
            <Link to={`/projects/${encodeURIComponent(p.key)}`}>
              <span className="project-tile" aria-hidden="true">{p.key.slice(0, 2)}</span>
              <span className="project-text"><span className="key">{p.key}</span> {p.name}</span>
              <span className="project-go" aria-hidden="true">→</span>
            </Link>
          </li>
        ))}
      </ul>
      <form className="panel inline" onSubmit={onSubmit}>
        <h2>{t("New project")}</h2>
        <label>
          {t("Key")}
          <input value={key} onChange={(e) => setKey(e.target.value.toUpperCase())} placeholder={t("KYB")} required maxLength={10} />
        </label>
        <label>
          {t("Project name")}
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder={t("Kyber")} required />
        </label>
        <button type="submit" disabled={create.isPending}>
          {t("Create project")}
        </button>
        {create.isError && (
          <p role="alert" className="error">
            {create.error.message}
          </p>
        )}
      </form>
    </div>
  );
}
