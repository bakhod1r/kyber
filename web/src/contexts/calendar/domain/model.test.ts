import { describe, expect, it } from "vitest";
import { audienceLabel, tally } from "./model";

describe("calendar model", () => {
  it("tallies answers", () => {
    expect(tally({})).toBe("");
    expect(tally({ responses: { a: "going", b: "going", c: "declined" } })).toBe("2 going · 1 declined");
  });
  it("names the audience", () => {
    expect(audienceLabel({ scope: "project", project_key: "KYB" })).toBe("KYB");
    expect(audienceLabel({ scope: "workspace" })).toBe("Everyone");
    expect(audienceLabel({ scope: "people" })).toBe("");
    expect(audienceLabel({})).toBe("");
  });
});
