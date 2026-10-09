import { act, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { FocusSession } from "../api";
import { mockApi, renderWithProviders } from "../test-utils";
import { FocusTimer, IssueFocus } from "./Focus";

const session = (over: Partial<FocusSession> = {}): FocusSession => ({
  id: "s-1", issue_key: "KYB-1", user_id: "u-1", state: "running", planned_minutes: 25, started_at: "2026-10-12T09:00:00Z",
  ended_at: null, focused_seconds: 60, remaining_seconds: 24 * 60, reason: "", ...over,
});

afterEach(() => vi.useRealTimers());

describe("Pomodoro (KYB-S41)", () => {
  it("AC1: the top bar counts down the running session and can pause it", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    let state: FocusSession = session();
    const calls = mockApi({
      "GET /api/v1/focus": () => ({ status: 200, body: { session: state } }),
      "POST /api/v1/focus/pause": () => {
        state = session({ state: "paused" });
        return { status: 200, body: state };
      },
    });
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    renderWithProviders(<FocusTimer />);
    const timer = await screen.findByRole("timer", { name: "Focus on KYB-1" });
    expect(timer).toHaveTextContent("24:00");
    await act(async () => vi.advanceTimersByTime(3000));
    expect(timer).toHaveTextContent("23:57");
    await user.click(screen.getByRole("button", { name: "Pause" }));
    expect(await screen.findByRole("button", { name: "Resume" })).toBeInTheDocument();
    expect(calls.some((c) => c.key === "POST /api/v1/focus/pause")).toBe(true);
  });

  it("AC1: nothing shows without a session", async () => {
    mockApi({ "GET /api/v1/focus": { status: 200, body: { session: null } } });
    renderWithProviders(<FocusTimer />);
    await act(async () => {});
    expect(screen.queryByRole("timer")).not.toBeInTheDocument();
  });

  it("AC2: starting on an issue explains a meeting conflict", async () => {
    mockApi({
      "GET /api/v1/focus": { status: 200, body: { session: null } },
      "GET /api/v1/issues/KYB-1/focus": { status: 200, body: { items: [], total_focused_seconds: 0 } },
      "POST /api/v1/focus": {
        status: 409,
        body: { type: "about:blank", title: "x", status: 409, code: "FOCUS_MEETING_CONFLICT", detail: 'you have a meeting at that time: "Planning" 09:20–09:50' },
      },
    });
    const user = userEvent.setup();
    renderWithProviders(<IssueFocus issueKey="KYB-1" />);
    await user.click(await screen.findByRole("button", { name: "Focus 25 min" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Planning");
  });

  it("AC3: the history lists sessions and the total focus time", async () => {
    mockApi({
      "GET /api/v1/focus": { status: 200, body: { session: null } },
      "GET /api/v1/issues/KYB-1/focus": {
        status: 200,
        body: {
          items: [
            session({ state: "completed", focused_seconds: 1500, ended_at: "2026-10-12T09:25:00Z", remaining_seconds: 0 }),
            session({ id: "s-2", state: "interrupted", reason: "meeting: Standup", focused_seconds: 420, ended_at: "2026-10-12T10:07:00Z" }),
          ],
          total_focused_seconds: 1920,
        },
      },
    });
    renderWithProviders(<IssueFocus issueKey="KYB-1" />);
    const list = await screen.findByRole("list", { name: "Focus history" });
    expect(within(list).getAllByRole("listitem")).toHaveLength(2);
    expect(list).toHaveTextContent("meeting: Standup");
    expect(screen.getByText("Total focus: 32 min")).toBeInTheDocument();
  });
});

describe("Pomodoro buttons", () => {
  it("are disabled while a session runs", async () => {
    mockApi({
      "GET /api/v1/focus": { status: 200, body: { session: session() } },
      "GET /api/v1/issues/KYB-2/focus": { status: 200, body: { items: [], total_focused_seconds: 0 } },
    });
    renderWithProviders(<IssueFocus issueKey="KYB-2" />);
    expect(await screen.findByText(/A focus session is running \(KYB-1\)/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Focus 25 min" })).toBeDisabled();
  });
});

describe("Pomodoro → eye break", () => {
  it("opens the guided eye break when the focus time is up", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    mockApi({ "GET /api/v1/focus": { status: 200, body: { session: session({ remaining_seconds: 2 }) } } });
    renderWithProviders(<FocusTimer />);
    await screen.findByRole("timer");
    await act(async () => vi.advanceTimersByTime(3000));
    expect(await screen.findByRole("dialog", { name: "Eye break" })).toBeInTheDocument();
  });

  it("can be opened any time from the top bar", async () => {
    mockApi({ "GET /api/v1/focus": { status: 200, body: { session: null } } });
    const user = userEvent.setup();
    renderWithProviders(<FocusTimer />);
    await user.click(await screen.findByRole("button", { name: "Eye break" }));
    expect(screen.getByRole("dialog", { name: "Eye break" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Skip break" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
