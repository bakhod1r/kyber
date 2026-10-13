// form: HTTP gateway to the forms API (infrastructure).
import { request, seg, type List } from "../../../shared/http";
import type { Form, FormInput, Submission } from "../domain/model";

export const formGateway = {
  forms: (key: string) => request<List<Form>>("GET", `/api/v1/projects/${seg(key)}/forms`).then((r) => r.items),
  createForm: (key: string, f: FormInput) => request<Form>("POST", `/api/v1/projects/${seg(key)}/forms`, f),
  redesignForm: (id: string, version: number, f: FormInput) => request<Form>("PUT", `/api/v1/forms/${seg(id)}`, { version, ...f }),
  deleteForm: (id: string) => request<undefined>("DELETE", `/api/v1/forms/${seg(id)}`),
  submit: (id: string, s: Submission) => request<{ issue_key: string }>("POST", `/api/v1/forms/${seg(id)}/submissions`, s),
};
