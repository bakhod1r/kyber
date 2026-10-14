// docs: HTTP gateway to the pages API (infrastructure).
import { request, seg, type List } from "../../../shared/http";
import type { DocComment, Page, PageInput, PageRevision } from "../domain/model";

export const docsGateway = {
  pages: (key: string) => request<List<Page>>("GET", `/api/v1/projects/${seg(key)}/docs`).then((r) => r.items),
  page: (id: string) => request<Page>("GET", `/api/v1/docs/${seg(id)}`),
  createPage: (key: string, p: PageInput) => request<Page>("POST", `/api/v1/projects/${seg(key)}/docs`, p),
  revisePage: (id: string, version: number, p: PageInput) => request<Page>("PUT", `/api/v1/docs/${seg(id)}`, { version, ...p }),
  deletePage: (id: string) => request<undefined>("DELETE", `/api/v1/docs/${seg(id)}`),
  history: (id: string) => request<List<PageRevision>>("GET", `/api/v1/docs/${seg(id)}/history`).then((r) => r.items),
  comments: (id: string) => request<List<DocComment>>("GET", `/api/v1/docs/${seg(id)}/comments`).then((r) => r.items),
  comment: (id: string, paragraph: number, quote: string, body: string) =>
    request<DocComment>("POST", `/api/v1/docs/${seg(id)}/comments`, { paragraph, quote, body }),
  removeComment: (id: string) => request<undefined>("DELETE", `/api/v1/doc-comments/${seg(id)}`),
};
