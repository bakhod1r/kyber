import { describe, expect, it } from "vitest";
import { paragraphs, placeComment, quoteOf } from "./model";

describe("doc paragraphs", () => {
  it("splits on blank lines but keeps code blocks whole", () => {
    expect(paragraphs("# Hi\n\nOne\ntwo\n\n\n```\na\n\nb\n```\n\nEnd\n")).toEqual(["# Hi", "One\ntwo", "```\na\n\nb\n```", "End"]);
    expect(paragraphs("  \n")).toEqual([]);
  });

  it("places a comment where its quote now is", () => {
    const paras = ["# Hi", "Intro", "Cache it here"];
    expect(placeComment(paras, { paragraph: 2, quote: "Cache it" })).toBe(2);
    expect(placeComment(["# Hi", "New", ...paras.slice(1)], { paragraph: 2, quote: "Cache it" })).toBe(3);
    expect(placeComment(["# Hi"], { paragraph: 2, quote: "Cache it" })).toBe(-1);
    expect(quoteOf("  " + "x".repeat(100))).toHaveLength(80);
  });
});
