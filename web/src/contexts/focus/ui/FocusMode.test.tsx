import { act, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { FocusSession } from "../application";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { FocusTimer, IssueFocus, announceIssueDone } from "./Focus";

const session = (over: Partial<FocusSession> = {}): FocusSession => ({
  id: "s-1", issue_key: "KYB-1", user_id: "u-1", state: "running", planned_minutes: 25, started_at: "2026-10-12T09:00:00Z",
  ended_at: null, focused_seconds: 60, remaining_seconds: 24 * 60, remaining_ms: 24 * 60_000, ends_at: null, server_time: "2026-10-12T09:01:00Z",
  reason: "", ...over,
});

beforeEach(() => localStorage.clear());
afterEach(() => vi.useRealTimers());

describe("Pomodoro precision (stopwatch)", () => {
  it("counts down from the server deadline, ignoring a skewed client clock and sub-second remainders", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    // The client clock is 1 hour ahead of the server; 90.4 s are left.
    vi.setSystemTime(new Date("2026-10-12T10:00:00.000Z"));
    mockApi({
      "GET /api/v1/focus": {
        status: 200,
        body: { session: session({ server_time: "2026-10-12T09:00:00.000Z", ends_at: "2026-10-12T09:01:30.400Z", remaining_seconds: 90, remaining_ms: 90_400 }) },
      },
    });
    renderWithProviders(<FocusTimer />);
    const timer = await screen.findByRole("timer");
    expect(timer).toHaveTextContent("01:31"); // ceil: a stopwatch never shows 0 before the end
    await act(async () => vi.advanceTimersByTime(400));
    expect(timer).toHaveTextContent("01:30");
    await act(async () => vi.advanceTimersByTime(60_000));
    expect(timer).toHaveTextContent("00:30");
  });
});

describe("Pomodoro mode on/off", () => {
  it("is on by default; turning it off hides the focus buttons and is remembered", async () => {
    mockApi({
      "GET /api/v1/focus": { status: 200, body: { session: null } },
      "GET /api/v1/issues/KYB-1/focus": { status: 200, body: { items: [], total_focused_seconds: 0 } },
    });
    const user = userEvent.setup();
    const { unmount } = renderWithProviders(
      <>
        <FocusTimer />
        <IssueFocus issueKey="KYB-1" />
      </>,
    );
    const toggle = await screen.findByRole("button", { name: "Pomodoro mode" });
    expect(toggle).toHaveAttribute("aria-pressed", "true");
    expect(await screen.findByRole("button", { name: "Focus 25 min" })).toBeInTheDocument();
    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-pressed", "false");
    expect(screen.queryByRole("button", { name: "Focus 25 min" })).not.toBeInTheDocument();
    expect(screen.getByText(/Pomodoro mode is off/)).toBeInTheDocument();
    unmount();
    renderWithProviders(<FocusTimer />);
    expect(await screen.findByRole("button", { name: "Pomodoro mode" })).toHaveAttribute("aria-pressed", "false");
  });
});

describe("Eye health", () => {
  it("with Pomodoro off, finishing a task asks for an eye exercise", async () => {
    localStorage.setItem("kyber.pomodoro", "off");
    mockApi({ "GET /api/v1/focus": { status: 200, body: { session: null } } });
    const user = userEvent.setup();
    renderWithProviders(<FocusTimer />);
    await screen.findByRole("button", { name: "Pomodoro mode" });
    act(() => announceIssueDone("KYB-7"));
    const ask = await screen.findByRole("alertdialog", { name: "Eye exercise" });
    expect(ask).toHaveTextContent("KYB-7");
    await user.click(screen.getByRole("button", { name: "Start exercise" }));
    expect(screen.getByRole("dialog", { name: "Eye break" })).toBeInTheDocument();
  });

  it("the question can be dismissed", async () => {
    localStorage.setItem("kyber.pomodoro", "off");
    mockApi({ "GET /api/v1/focus": { status: 200, body: { session: null } } });
    const user = userEvent.setup();
    renderWithProviders(<FocusTimer />);
    await screen.findByRole("button", { name: "Pomodoro mode" });
    act(() => announceIssueDone("KYB-7"));
    await user.click(await screen.findByRole("button", { name: "Not now" }));
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });

  it("with Pomodoro on, finishing a task does not interrupt (the exercise comes at the break)", async () => {
    mockApi({ "GET /api/v1/focus": { status: 200, body: { session: null } } });
    renderWithProviders(<FocusTimer />);
    await screen.findByRole("button", { name: "Pomodoro mode" });
    act(() => announceIssueDone("KYB-7"));
    expect(screen.queryByRole("alertdialog")).not.toBeInTheDocument();
  });
});
