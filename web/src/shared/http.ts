// Package-wide HTTP client: JSON requests, RFC 9457 problems as ApiError.
/** RFC 9457 problem details as returned by the API (application/problem+json). */
export type Problem = { type: string; title: string; status: number; code: string; detail?: string; numeric_code?: number };

export class ApiError extends Error {
  constructor(
    readonly status: number,
    message: string,
    /** Stable Kyber error code (e.g. ISSUE_CONFLICT) — branch on this, not on message text. */
    readonly code: string = "",
  ) {
    super(message);
    this.name = "ApiError";
  }
}

export function isProblem(v: unknown): v is Problem {
  return typeof v === "object" && v !== null && "title" in v && typeof v.title === "string";
}

export async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    credentials: "same-origin",
    headers: { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await res.text();
  const data: unknown = text ? JSON.parse(text) : undefined;
  if (!res.ok) {
    if (isProblem(data)) throw new ApiError(res.status, data.detail || data.title, data.code);
    throw new ApiError(res.status, `request failed (${res.status})`);
  }
  return data as T;
}

/** requestText fetches a text body (release notes are Markdown). */
export async function requestText(path: string): Promise<string> {
  const res = await fetch(path, { credentials: "same-origin" });
  const text = await res.text();
  if (!res.ok) {
    const data: unknown = text ? JSON.parse(text) : undefined;
    if (isProblem(data)) throw new ApiError(res.status, data.detail || data.title, data.code);
    throw new ApiError(res.status, `request failed (${res.status})`);
  }
  return text;
}

export const seg = encodeURIComponent;
export type List<T> = { items: T[] };
