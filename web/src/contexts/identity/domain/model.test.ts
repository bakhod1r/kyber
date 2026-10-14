import { describe, expect, it } from "vitest";
import { authorizeRequest, safeNext } from "./model";

describe("safeNext", () => {
  it.each([
    ["/oauth/authorize?client_id=c", "/oauth/authorize?client_id=c"],
    [null, "/"],
    ["https://evil.example", "/"],
    ["//evil.example", "/"],
    ["/\\evil.example", "/"],
  ])("%s → %s", (raw, want) => expect(safeNext(raw)).toBe(want));
});

describe("authorizeRequest", () => {
  it("reads an OAuth authorization request", () => {
    const q = new URLSearchParams("response_type=code&client_id=c-1&redirect_uri=http%3A%2F%2Flocalhost%3A1%2Fcb&code_challenge=abc&code_challenge_method=S256&state=s");
    expect(authorizeRequest(q)).toEqual({ client_id: "c-1", redirect_uri: "http://localhost:1/cb", code_challenge: "abc", code_challenge_method: "S256", state: "s" });
  });
  it("rejects incomplete or non-code requests", () => {
    expect(authorizeRequest(new URLSearchParams("response_type=token&client_id=c&redirect_uri=x&code_challenge=a"))).toBeNull();
    expect(authorizeRequest(new URLSearchParams("response_type=code&client_id=c&redirect_uri=x"))).toBeNull();
  });
});
