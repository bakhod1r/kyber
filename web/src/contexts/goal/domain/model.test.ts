import { describe, expect, it } from "vitest";
import { daysLeft, deadlineText, statusLabel } from "./model";

describe("goal model", () => {
  it("counts days to the target", () => {
    expect(daysLeft({ target_date: null }, "2026-10-13")).toBeNull();
    expect(daysLeft({ target_date: "2026-10-20" }, "2026-10-13")).toBe(7);
  });

  it("describes the deadline", () => {
    const t = "2026-10-13";
    expect(deadlineText({ target_date: null, status: "on_track" }, t)).toBe("No target date");
    expect(deadlineText({ target_date: "2026-10-13", status: "on_track" }, t)).toBe("Due today");
    expect(deadlineText({ target_date: "2026-10-14", status: "at_risk" }, t)).toBe("1 day left");
    expect(deadlineText({ target_date: "2026-10-20", status: "on_track" }, t)).toBe("7 days left");
    expect(deadlineText({ target_date: "2026-10-11", status: "off_track" }, t)).toBe("2 days overdue");
    expect(deadlineText({ target_date: "2026-10-12", status: "off_track" }, t)).toBe("1 day overdue");
    expect(deadlineText({ target_date: "2026-10-01", status: "done" }, t)).toBe("Target 2026-10-01");
    expect(statusLabel("at_risk")).toBe("At risk");
  });
});
