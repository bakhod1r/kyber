import { describe, expect, it } from "vitest";
import { completeJql, projectQuery, withoutOrder } from "./model";

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

describe("JQL autocomplete", () => {
  const opts = (text: string, extra = {}) => completeJql(text, extra).options;
  it("suggests fields at the start and after AND/OR", () => {
    expect(opts("")).toContain("project");
    expect(opts("statu")).toEqual(["status", "statusCategory"]);
    expect(opts("type = bug AND lab")).toEqual(["labels"]);
    expect(completeJql("type = bug AND lab", {}).from).toBe(15);
  });
  it("suggests operators after a field", () => {
    expect(opts("status ")).toEqual(expect.arrayContaining(["=", "!=", "in", "is"]));
  });
  it("suggests values after an operator, including dynamic ones", () => {
    expect(opts("type = b")).toEqual(["bug"]);
    expect(opts("assignee = ")).toContain("currentUser()");
    expect(opts("labels = ", { labels: ["ui", "backend"] })).toEqual(["ui", "backend"]);
    expect(opts("labels in (ui, ", { labels: ["ui", "backend"] })).toEqual(["ui", "backend"]);
    expect(opts("resolution is ")).toEqual(["EMPTY"]);
  });
  it("suggests keywords after a complete clause", () => {
    expect(opts("type = bug ")).toEqual(["AND", "OR", "ORDER BY"]);
    expect(opts("type = bug o")).toEqual(["OR", "ORDER BY"]);
  });
  it("applies a suggestion in place", () => {
    const c = completeJql("type = bug AND lab", {});
    expect(c.apply("labels")).toBe("type = bug AND labels ");
  });
});
