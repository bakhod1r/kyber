import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { api } from "../api";

export function ProjectsPage() {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const projects = useQuery({ queryKey: ["projects"], queryFn: api.projects });
  const [key, setKey] = useState("");
  const [name, setName] = useState("");
  const create = useMutation({
    mutationFn: () => api.createProject(key, name),
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
      <h1>Projects</h1>
      {projects.isPending && <p className="muted">Loading…</p>}
      {projects.isError && <p role="alert" className="error">{projects.error.message}</p>}
      {projects.data?.length === 0 && <p className="muted">No projects yet — create the first one below.</p>}
      <ul className="project-list">
        {projects.data?.map((p) => (
          <li key={p.id}>
            <Link to={`/projects/${encodeURIComponent(p.key)}`}>
              <span className="key">{p.key}</span> {p.name}
            </Link>
          </li>
        ))}
      </ul>
      <form className="panel inline" onSubmit={onSubmit}>
        <h2>New project</h2>
        <label>
          Key
          <input value={key} onChange={(e) => setKey(e.target.value.toUpperCase())} placeholder="KYB" required maxLength={10} />
        </label>
        <label>
          Project name
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="Kyber" required />
        </label>
        <button type="submit" disabled={create.isPending}>
          Create project
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
