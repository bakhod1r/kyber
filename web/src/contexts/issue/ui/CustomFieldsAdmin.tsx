import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, useState } from "react";
import { ApiError } from "../../../shared/http";
import { CUSTOM_FIELD_TYPES, type CustomField, type CustomFieldType, issueUseCases, parseOptions } from "../application";

/** CustomFieldsAdmin: the project's custom fields (Jira's field configuration), for project admins. */
export function CustomFieldsAdmin({ projectKey }: { projectKey: string }) {
  const qc = useQueryClient();
  const key = ["custom-fields", projectKey];
  const fields = useQuery({ queryKey: key, queryFn: () => issueUseCases.customFields(projectKey) });
  const [name, setName] = useState("");
  const [type, setType] = useState<CustomFieldType>("text");
  const [options, setOptions] = useState("");
  const [notice, setNotice] = useState<{ kind: "ok" | "error"; text: string } | null>(null);
  const done = (text: string) => ({
    onSuccess: () => {
      setNotice({ kind: "ok" as const, text });
      return qc.invalidateQueries({ queryKey: key });
    },
    onError: (err: Error) =>
      setNotice({ kind: "error" as const, text: err instanceof ApiError && err.status === 403 ? "Only project admins can change fields." : err.message }),
  });
  const create = useMutation({
    mutationFn: () => issueUseCases.createCustomField(projectKey, { name, type, options: type === "select" ? parseOptions(options) : [] }),
    ...done("Field added"),
    onSuccess: () => {
      setName("");
      setOptions("");
      setNotice({ kind: "ok", text: "Field added" });
      return qc.invalidateQueries({ queryKey: key });
    },
  });
  const remove = useMutation({ mutationFn: (f: CustomField) => issueUseCases.deleteCustomField(f.id), ...done("Field deleted") });
  const rename = useMutation({
    mutationFn: ({ f, to, opts }: { f: CustomField; to: string; opts: string[] }) => issueUseCases.editCustomField(f.id, f.version, { name: to, options: opts }),
    ...done("Field saved"),
  });

  function onCreate(e: FormEvent) {
    e.preventDefault();
    if (name.trim()) create.mutate();
  }
  if (fields.isPending) return <p className="muted">Loading…</p>;
  if (fields.isError) return <p role="alert" className="error">{fields.error.message}</p>;
  return (
    <section className="panel" aria-label="Custom fields">
      <h2>Custom fields</h2>
      <p className="muted">Fields every issue of this project can have. Search them with JQL by name, e.g. "Tier" = Gold.</p>
      {fields.data.length === 0 ? (
        <p className="muted">No custom fields yet.</p>
      ) : (
        <table className="versions-table">
          <tbody>
            {fields.data.map((f) => (
              <FieldRow key={f.id + f.version} field={f} onSave={(to, opts) => rename.mutate({ f, to, opts })} onDelete={() => remove.mutate(f)} />
            ))}
          </tbody>
        </table>
      )}
      <form className="row" onSubmit={onCreate}>
        <label>
          Field name
          <input value={name} onChange={(e) => setName(e.target.value)} maxLength={60} />
        </label>
        <label>
          Field type
          <select value={type} onChange={(e) => setType(e.target.value as CustomFieldType)}>
            {CUSTOM_FIELD_TYPES.map((t) => (
              <option key={t.id} value={t.id}>
                {t.label}
              </option>
            ))}
          </select>
        </label>
        {type === "select" && (
          <label>
            Options
            <input value={options} onChange={(e) => setOptions(e.target.value)} placeholder="Gold, Silver, Bronze" />
          </label>
        )}
        <button type="submit" disabled={create.isPending || !name.trim()}>
          Add field
        </button>
      </form>
      {notice && (
        <p role={notice.kind === "error" ? "alert" : "status"} className={notice.kind === "error" ? "error" : "muted"}>
          {notice.text}
        </p>
      )}
    </section>
  );
}

function FieldRow({ field, onSave, onDelete }: { field: CustomField; onSave: (name: string, options: string[]) => void; onDelete: () => void }) {
  const [name, setName] = useState(field.name);
  const [options, setOptions] = useState(field.options.join(", "));
  return (
    <tr>
      <td>
        <input aria-label={`Name of ${field.name}`} value={name} onChange={(e) => setName(e.target.value)} maxLength={60} />
      </td>
      <td>{CUSTOM_FIELD_TYPES.find((t) => t.id === field.type)?.label}</td>
      <td>
        {field.type === "select" && <input aria-label={`Options of ${field.name}`} value={options} onChange={(e) => setOptions(e.target.value)} />}
      </td>
      <td className="row">
        <button type="button" className="ghost" onClick={() => onSave(name, field.type === "select" ? parseOptions(options) : [])}>
          Save {field.name}
        </button>
        <button type="button" className="ghost" onClick={onDelete}>
          Delete {field.name}
        </button>
      </td>
    </tr>
  );
}
