import { describe, expect, it } from "vitest";
import { ApiError } from "./http";
import { mockApi } from "./test/test-utils";
import { identityUseCases } from "../contexts/identity/application";
import { issueUseCases } from "../contexts/issue/application";
import { projectUseCases } from "../contexts/project/application";

describe("api client", () => {
  it("sends JSON with the CSRF-safe content type", async () => {
    const calls = mockApi({ "POST /api/v1/projects": { status: 201, body: { id: "1", key: "KYB", name: "Kyber" } } });
    const p = await projectUseCases.createProject("KYB", "Kyber");
    expect(p.key).toBe("KYB");
    expect(calls[0]?.headers.get("Content-Type")).toBe("application/json");
    expect(calls[0]?.body).toEqual({ key: "KYB", name: "Kyber" });
  });

  it("asks for problems in the chosen language", async () => {
    localStorage.setItem("kyber.lang", "uz");
    try {
      const calls = mockApi({ "GET /api/v1/projects": { status: 200, body: { items: [] } } });
      await projectUseCases.projects();
      expect(calls[0]?.headers.get("Accept-Language")).toBe("uz");
    } finally {
      localStorage.removeItem("kyber.lang");
    }
  });

  it("surfaces RFC 9457 problems: detail as message, code for branching", async () => {
    mockApi({
      "POST /api/v1/issues/KYB-1/transitions": {
        status: 409,
        body: { type: "about:blank", title: "This status change is not allowed.", status: 409, code: "ISSUE_TRANSITION_NOT_ALLOWED", detail: "transition not allowed by workflow" },
      },
    });
    const err = await issueUseCases.transition("KYB-1", "done").catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect((err as ApiError).status).toBe(409);
    expect((err as ApiError).code).toBe("ISSUE_TRANSITION_NOT_ALLOWED");
    expect((err as ApiError).message).toBe("transition not allowed by workflow");
  });

  it("falls back to the localized title when a problem has no detail", async () => {
    mockApi({ "GET /api/v1/me": { status: 500, body: { type: "about:blank", title: "Nimadir noto'g'ri ketdi.", status: 500, code: "INTERNAL" } } });
    const err = (await identityUseCases.me().catch((e: unknown) => e)) as ApiError;
    expect(err.message).toBe("Nimadir noto'g'ri ketdi.");
  });

  it("handles empty 204 responses", async () => {
    mockApi({ "POST /api/v1/auth/logout": { status: 204 } });
    await expect(identityUseCases.logout()).resolves.toBeUndefined();
  });

  it("encodes path segments", async () => {
    const calls = mockApi({ "GET /api/v1/projects/A%2FB/issues": { status: 200, body: { items: [] } } });
    await issueUseCases.issues("A/B");
    expect(calls[0]?.key).toBe("GET /api/v1/projects/A%2FB/issues");
  });
});
