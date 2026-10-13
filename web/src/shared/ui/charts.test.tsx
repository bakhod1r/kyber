import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { BarList, LineChart, niceMax } from "./charts";

describe("chart primitives", () => {
  it("niceMax rounds the axis up to a clean value", () => {
    expect(niceMax(0)).toBe(1);
    expect(niceMax(7)).toBe(8);
    expect(niceMax(23)).toBe(25);
    expect(niceMax(130)).toBe(150);
  });

  it("BarList scales bars to the largest value and labels the tip", () => {
    render(<BarList title="Issues by type" items={[{ label: "Story", value: 5 }, { label: "Bug", value: 10 }]} />);
    const fig = screen.getByRole("figure", { name: "Issues by type" });
    const bars = fig.querySelectorAll("[data-bar]");
    expect(bars).toHaveLength(2);
    expect(Number(bars[1]!.getAttribute("data-width"))).toBeGreaterThan(Number(bars[0]!.getAttribute("data-width")) * 1.9);
    expect(within(fig).getAllByText("10").length).toBeGreaterThan(0);
  });

  it("LineChart shows a legend for two series and a crosshair tooltip on hover", () => {
    render(
      <LineChart
        title="Created vs resolved"
        series={[
          { name: "Created", slot: 1, points: [{ x: new Date("2026-05-01"), y: 1 }, { x: new Date("2026-05-02"), y: 4 }] },
          { name: "Resolved", slot: 2, points: [{ x: new Date("2026-05-01"), y: 0 }, { x: new Date("2026-05-02"), y: 2 }] },
        ]}
      />,
    );
    const fig = screen.getByRole("figure", { name: "Created vs resolved" });
    expect(within(fig).getByRole("list", { name: "Legend" })).toHaveTextContent("CreatedResolved");
    const plot = fig.querySelector("[data-plot]")!;
    plot.getBoundingClientRect = () => ({ left: 0, top: 0, width: 800, height: 260, right: 800, bottom: 260, x: 0, y: 0, toJSON: () => ({}) });
    fireEvent.pointerMove(plot, { clientX: 790, clientY: 100 });
    const tip = within(fig).getByRole("status");
    expect(tip).toHaveTextContent("4");
    expect(tip).toHaveTextContent("Created");
    expect(tip).toHaveTextContent("2");
  });
});

describe("robustness", () => {
  it("LineChart ignores malformed points instead of crashing", () => {
    render(<LineChart title="T" series={[{ name: "A", slot: 1, points: [{ x: new Date("nope"), y: 1 }, { x: new Date("2026-05-02"), y: 3 }] }]} />);
    expect(screen.getByRole("figure", { name: "T" })).toBeInTheDocument();
  });
});
