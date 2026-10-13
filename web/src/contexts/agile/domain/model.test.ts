import { describe, expect, it } from "vitest";
import { addColumn, assignStatus, type BoardColumn, boardProblems, columnOf, moveColumn } from "./model";

const cols: BoardColumn[] = [
  { name: "To Do", statuses: ["todo"], min: 0, max: 0 },
  { name: "Doing", statuses: ["in_progress"], min: 0, max: 3 },
  { name: "Done", statuses: ["done"], min: 0, max: 0 },
];

describe("board configuration rules", () => {
  it("moves columns within range", () => {
    expect(moveColumn(cols, 2, -1).map((c) => c.name)).toEqual(["To Do", "Done", "Doing"]);
    expect(moveColumn(cols, 0, -1)).toBe(cols);
    expect(moveColumn(cols, 2, 1)).toBe(cols);
    expect(moveColumn(cols, 7, -1)).toBe(cols);
  });
  it("keeps a status in one column at most", () => {
    const moved = assignStatus(cols, "todo", 1);
    expect(moved.map((c) => c.statuses)).toEqual([[], ["in_progress", "todo"], ["done"]]);
    expect(columnOf(moved, "todo")).toBe(1);
    expect(columnOf(assignStatus(cols, "done", -1), "done")).toBe(-1);
  });
  it("adds uniquely named columns", () => {
    const one = addColumn(cols);
    const two = addColumn(one);
    expect(two.slice(3).map((c) => c.name)).toEqual(["New column", "New column 2"]);
  });
  it("explains what the server would refuse", () => {
    expect(boardProblems("Flow", cols)).toEqual([]);
    expect(boardProblems(" ", [])).toEqual(["The board needs a name.", "A board needs at least one column."]);
    expect(
      boardProblems("x", [
        { name: "A", statuses: [], min: 0, max: 0 },
        { name: "a ", statuses: [], min: 0, max: 0 },
        { name: "", statuses: [], min: 0, max: 0 },
        { name: "B", statuses: [], min: 5, max: 2 },
      ]),
    ).toEqual(['Two columns are called "a".', "Every column needs a name.", '"B": the minimum is above the maximum.']);
  });
});
