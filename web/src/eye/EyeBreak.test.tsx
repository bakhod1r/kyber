import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { EyeBreak } from "./EyeBreak";

beforeEach(() => vi.useFakeTimers({ shouldAdvanceTime: true }));
afterEach(() => vi.useRealTimers());

describe("Eye break (from Sharingan)", () => {
  it("guides the eyes step by step and finishes", async () => {
    const onClose = vi.fn();
    render(<EyeBreak onClose={onClose} />);
    const dialog = screen.getByRole("dialog", { name: "Eye break" });
    expect(dialog).toHaveAttribute("aria-modal", "true");
    const say = screen.getByRole("status");
    expect(say).toHaveTextContent("Look at something 20 feet");
    await act(async () => vi.advanceTimersByTime(26_000));
    expect(say).toHaveTextContent("Look right");
    const guide = screen.getByTestId("eye-guide");
    expect(guide.getAttribute("transform")).toMatch(/translate\(\d/); // moved to the right
    await act(async () => vi.advanceTimersByTime(100_000));
    expect(screen.getByRole("heading", { name: "Well done — your eyes thank you" })).toBeInTheDocument();
    await userEvent.setup({ advanceTimers: vi.advanceTimersByTime }).click(screen.getByRole("button", { name: "Back to work" }));
    expect(onClose).toHaveBeenCalledWith("done");
  });

  it("can be skipped, also with Escape", async () => {
    const onClose = vi.fn();
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<EyeBreak onClose={onClose} />);
    await user.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalledWith("skipped");
    await user.click(screen.getByRole("button", { name: "Skip break" }));
    expect(onClose).toHaveBeenCalledTimes(2);
  });
});
