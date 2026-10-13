import { describe, expect, it } from "vitest";
import { type Call, fromFraction, isPolite, me, opens, peersOf, presenter, toFraction } from "./model";

const call: Call = {
  id: "k-1", conversation_id: "c-1", started_by: "a", started_at: "", live: true,
  participants: [
    { user_id: "a", joined_at: "", audio: true, video: false, screen: false },
    { user_id: "b", joined_at: "", audio: false, video: true, screen: true },
  ],
};

describe("call domain", () => {
  it("knows the peers, the presenter and me", () => {
    expect(peersOf(call, "a")).toEqual(["b"]);
    expect(peersOf({ ...call, live: false }, "a")).toEqual([]);
    expect(presenter(call)).toBe("b");
    expect(me(call, "b")?.video).toBe(true);
    expect(me(call, "z")).toBeUndefined();
  });

  it("lets the one who joined first open the connection", () => {
    const c: Call = { ...call, participants: [{ ...call.participants[0]!, joined_at: "2026-10-13T09:00:00Z" }, { ...call.participants[1]!, joined_at: "2026-10-13T09:01:00Z" }] };
    expect(opens(c, "a", "b")).toBe(true);
    expect(opens(c, "b", "a")).toBe(false);
    expect(opens(call, "a", "b")).not.toBe(opens(call, "b", "a")); // same time: by id
    expect(opens(call, "a", "z")).toBe(false);
  });

  it("makes exactly one of two peers polite", () => {
    expect(isPolite("a", "b")).not.toBe(isPolite("b", "a"));
  });
});

describe("follow the presenter", () => {
  it("shares scroll positions as fractions", () => {
    expect(toFraction(250, 1500, 1000)).toBe(0.5);
    expect(toFraction(10, 500, 1000)).toBe(0); // nothing to scroll
    expect(fromFraction(0.5, 3000, 1000)).toBe(1000);
  });
});
