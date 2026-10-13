// search: HTTP gateway to the search API (infrastructure).
import { request, seg, type List } from "../../../shared/http";
import type { Filter, FilterInput, SearchResult } from "../domain/model";

export const searchGateway = {
  search: (jql: string) => request<SearchResult>("GET", `/api/v1/search?jql=${encodeURIComponent(jql)}`),
  filters: () => request<List<Filter>>("GET", "/api/v1/filters").then((r) => r.items),
  createFilter: (f: FilterInput) => request<Filter>("POST", "/api/v1/filters", f),
  updateFilter: (id: string, version: number, f: FilterInput) => request<Filter>("PUT", `/api/v1/filters/${seg(id)}`, { version, ...f }),
  deleteFilter: (id: string) => request<undefined>("DELETE", `/api/v1/filters/${seg(id)}`),
};
