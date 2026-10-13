import { describe, expect, it } from "vitest";
import { projectQuery, withoutOrder } from "./model";

describe("project queries", () => {
  it("combines the project, quick filters, a saved filter and the order", () => {
    expect(projectQuery("KYB", [])).toBe('project = "KYB"');
    expect(projectQuery("KYB", ["mine", "nope", "open"], "type = bug ORDER BY priority DESC", "key DESC")).toBe(
      'project = "KYB" AND (assignee = currentUser()) AND (statusCategory != done) AND (type = bug) ORDER BY key DESC',
    );
    expect(projectQuery("KYB", [], "order by key")).toBe('project = "KYB"');
  });
  it("drops ORDER BY, whatever its case", () => {
    expect(withoutOrder("a = b Order  By c")).toBe("a = b");
    expect(withoutOrder("")).toBe("");
  });
});
