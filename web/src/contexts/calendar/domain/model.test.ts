import { describe, expect, it } from "vitest";
import { audienceLabel, localInput, minutesOf, tally, when } from "./model";

describe("calendar model", () => {
  const m = { starts_at: "2026-10-17T15:00:00Z", ends_at: "2026-10-17T16:30:00Z" };
  it("shows times in a time zone, naming it", () => {
    expect(when(m, "Asia/Tashkent")).toBe("Sat, 17 Oct, 20:00 – 21:30 (Asia/Tashkent)");
    expect(when(m, "UTC")).toBe("Sat, 17 Oct, 15:00 – 16:30 (UTC)");
  });
  it("fills the reschedule form in the zone", () => {
    expect(localInput(m.starts_at, "Asia/Tashkent")).toBe("2026-10-17T20:00");
    expect(localInput("2026-01-05T23:05:00Z", "America/New_York")).toBe("2026-01-05T18:05");
    expect(minutesOf(m)).toBe(90);
  });
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
