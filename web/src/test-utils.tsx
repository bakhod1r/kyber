import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render } from "@testing-library/react";
import type { ReactElement } from "react";
import { MemoryRouter } from "react-router-dom";
import { vi } from "vitest";

export type Route = { status: number; body?: unknown };
type Handler = (init: RequestInit | undefined) => Route;

/** Stubs fetch with "METHOD /path" handlers and records every call. */
export function mockApi(routes: Record<string, Handler | Route>) {
  const calls: { key: string; body: unknown; headers: Headers }[] = [];
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(String(input), "http://localhost");
    const key = `${init?.method ?? "GET"} ${url.pathname}${url.search}`;
    calls.push({ key, body: init?.body ? JSON.parse(String(init.body)) : undefined, headers: new Headers(init?.headers) });
    const route = routes[key];
    if (!route) return new Response(JSON.stringify({ error: `unmocked ${key}` }), { status: 500 });
    const { status, body } = typeof route === "function" ? route(init) : route;
    return new Response(body === undefined ? null : JSON.stringify(body), { status });
  });
  vi.stubGlobal("fetch", fetchMock);
  return calls;
}

export function renderWithProviders(ui: ReactElement, path = "/") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[path]}>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}
