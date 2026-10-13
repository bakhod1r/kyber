import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
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

  it("invites a whole project and answers invitations", async () => {
    const team = { ...meeting, id: "m-2", title: "Review", organizer_id: "u-9", scope: "project", project_key: "KYB", attendee_ids: ["u-1", "u-2", "u-9"], responses: { "u-2": "maybe" } };
    const calls = mockApi({
      "GET /api/v1/meetings": { status: 200, body: { items: [team] } },
      "GET /api/v1/me": { status: 200, body: { id: "u-1", email: "a@x.uz", name: "Ali" } },
      "GET /api/v1/projects": { status: 200, body: { items: [{ id: "p", key: "KYB", name: "Kyber" }] } },
      "GET /api/v1/projects/KYB/members": { status: 200, body: { items: [] } },
      "POST /api/v1/meetings": { status: 201, body: team },
      "PUT /api/v1/meetings/m-2/response": { status: 200, body: { ...team, responses: { "u-1": "going", "u-2": "maybe" } } },
    });
    const user = userEvent.setup();
    renderWithProviders(<MeetingsPage />);
    const table = await screen.findByRole("list", { name: "Meetings" });
    expect(await within(table).findByText("KYB")).toBeInTheDocument();
    expect(within(table).getByText(/1 maybe/)).toBeInTheDocument();
    const answers = within(table).getByRole("group", { name: "Answer Review" });
    await user.click(within(answers).getByRole("button", { name: "Going" }));
    expect(calls.find((c) => c.key === "PUT /api/v1/meetings/m-2/response")?.body).toEqual({ response: "going" });

    await user.type(screen.getByLabelText("Title"), "Sprint review");
    await user.type(screen.getByLabelText("Starts"), "2026-10-13T15:00");
    await user.selectOptions(screen.getByLabelText("Invite"), "project:KYB");
    expect(screen.getByText(/reminded 15 minutes before/)).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Schedule" }));
    expect(calls.find((c) => c.key === "POST /api/v1/meetings")?.body).toMatchObject({ scope: "project", project_key: "KYB", attendee_ids: [] });

    await user.selectOptions(screen.getByLabelText("Invite"), "workspace");
    await user.type(screen.getByLabelText("Title"), "All hands");
    await user.click(screen.getByRole("button", { name: "Schedule" }));
    const last = calls.filter((c) => c.key === "POST /api/v1/meetings").at(-1)?.body as Record<string, unknown>;
    expect(last.scope).toBe("workspace");
    expect(last.project_key).toBeUndefined();
  });
});
