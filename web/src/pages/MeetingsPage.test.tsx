import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../test-utils";
import { MeetingsPage } from "./MeetingsPage";

const meeting = { id: "m-1", title: "Planning", starts_at: "2026-10-12T09:00:00Z", ends_at: "2026-10-12T10:00:00Z", organizer_id: "u-1", attendee_ids: ["u-1", "u-2"] };

describe("Time table (KYB-S42)", () => {
  it("lists my meetings and schedules a new one with attendees", async () => {
    const calls = mockApi({
      "GET /api/v1/meetings": { status: 200, body: { items: [meeting] } },
      "GET /api/v1/me": { status: 200, body: { id: "u-1", email: "a@x.uz", name: "Ali" } },
      "GET /api/v1/projects": { status: 200, body: { items: [{ id: "p", key: "KYB", name: "Kyber" }] } },
      "GET /api/v1/projects/KYB/members": { status: 200, body: { items: [
        { user_id: "u-1", email: "a@x.uz", name: "Ali", role: "admin" }, { user_id: "u-2", email: "b@x.uz", name: "Bobur", role: "member" }] } },
      "POST /api/v1/meetings": { status: 201, body: meeting },
      "DELETE /api/v1/meetings/m-1": { status: 204 },
    });
    const user = userEvent.setup();
    renderWithProviders(<MeetingsPage />);
    const table = await screen.findByRole("list", { name: "Meetings" });
    expect(await within(table).findByText("Planning")).toBeInTheDocument();
    expect(within(table).getByText(/2 people/)).toBeInTheDocument();

    await user.type(screen.getByLabelText("Title"), "Retro");
    await user.type(screen.getByLabelText("Starts"), "2026-10-13T15:00");
    await user.clear(screen.getByLabelText("Minutes"));
    await user.type(screen.getByLabelText("Minutes"), "45");
    await user.click(await screen.findByRole("checkbox", { name: "Bobur" }));
    await user.click(screen.getByRole("button", { name: "Schedule" }));
    const body = calls.find((c) => c.key === "POST /api/v1/meetings")?.body as { title: string; attendee_ids: string[]; starts_at: string; ends_at: string };
    expect(body.title).toBe("Retro");
    expect(body.attendee_ids).toEqual(["u-2"]);
    expect(new Date(body.ends_at).getTime() - new Date(body.starts_at).getTime()).toBe(45 * 60_000);

    await user.click(within(table).getByRole("button", { name: "Cancel Planning" }));
    expect(calls.some((c) => c.key === "DELETE /api/v1/meetings/m-1")).toBe(true);
  });
});
