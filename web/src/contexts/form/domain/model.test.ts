import { describe, expect, it } from "vitest";
import { blankQuestion, missingAnswers } from "./model";

describe("form model", () => {
  it("lists unanswered required questions", () => {
    const form = { questions: [{ id: "q1", label: "Steps", required: true, long: true }, { id: "q2", label: "Browser", required: false, long: false }, { label: "New", required: true, long: false }] };
    expect(missingAnswers(form, { q1: "  ", q2: "" })).toEqual(["Steps", "New"]);
    expect(missingAnswers(form, { q1: "x", "": "y" })).toEqual([]);
    expect(blankQuestion()).toEqual({ label: "", required: false, long: false });
  });
});
