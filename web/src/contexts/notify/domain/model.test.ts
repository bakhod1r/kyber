import { describe, expect, it } from "vitest";
import { notificationTarget } from "./model";

describe("notificationTarget", () => {
  it("leads to the issue, the project's announcements or home", () => {
    expect(notificationTarget({ kind: "assigned", issue_key: "MY-APP-12" })).toBe("/projects/MY-APP?issue=MY-APP-12");
    expect(notificationTarget({ kind: "announcement", issue_key: "", project_key: "KYB" })).toBe("/projects/KYB/announcements");
    expect(notificationTarget({ kind: "announcement", issue_key: "", project_key: "" })).toBe("/");
  });
});
