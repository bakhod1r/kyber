import { describe, expect, it } from "vitest";
import { ApiError, api } from "./api";
import { mockApi } from "./test-utils";

describe("api client", () => {
  it("sends JSON with the CSRF-safe content type", async () => {
    const calls = mockApi({ "POST /api/v1/projects": { status: 201, body: { id: "1", key: "KYB", name: "Kyber" } } });
    const p = await api.createProject("KYB", "Kyber");
    expect(p.key).toBe("KYB");
    expect(calls[0]?.headers.get("Content-Type")).toBe("application/json");
    expect(calls[0]?.body).toEqual({ key: "KYB", name: "Kyber" });
  });

  it("surfaces the server error message and status", async () => {
    mockApi({ "POST /api/v1/issues/KYB-1/transitions": { status: 409, body: { error: "transition not allowed by workflow" } } });
    const err = await api.transition("KYB-1", "done").catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(409);
    expect((err as ApiError).message).toBe("transition not allowed by workflow");
  });

  it("handles empty 204 responses", async () => {
    mockApi({ "POST /api/v1/auth/logout": { status: 204 } });
    await expect(api.logout()).resolves.toBeUndefined();
  });

  it("encodes path segments", async () => {
    const calls = mockApi({ "GET /api/v1/projects/A%2FB/issues": { status: 200, body: { items: [] } } });
    await api.issues("A/B");
    expect(calls[0]?.key).toBe("GET /api/v1/projects/A%2FB/issues");
  });
});
