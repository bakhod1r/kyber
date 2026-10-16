import { screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { Members } from "./Members";

const member = { user_id: "u-1", email: "aziza@x.uz", name: "Aziza", role: "admin" };
const pending = { id: "i-1", email: "dan@x.uz", role: "member", invited_by: "u-1", created_at: "2026-10-15T09:00:00Z", expires_at: "2026-10-22T09:00:00Z" };

describe("Members (B1 invitations)", () => {
  it("invites someone without an account, shows the link when no email was sent, and revokes it", async () => {
    let invitations: (typeof pending)[] = [];
    const calls = mockApi({
      "GET /api/v1/projects/KYB/members": { status: 200, body: { items: [member] } },
      "GET /api/v1/projects/KYB/invitations": () => ({ status: 200, body: { items: invitations } }),
      "POST /api/v1/projects/KYB/invitations": () => {
        invitations = [pending];
        return { status: 201, body: { added: false, invitation: pending, link: "https://kyber.test/invite/abc", emailed: false } };
      },
      "DELETE /api/v1/projects/KYB/invitations/i-1": () => {
        invitations = [];
        return { status: 204 };
      },
    });
    renderWithProviders(<Members projectKey="KYB" />);
    await userEvent.type(await screen.findByLabelText("Invite by email"), "dan@x.uz");
    await userEvent.click(screen.getByRole("button", { name: "Invite" }));
    expect(await screen.findByRole("status")).toHaveTextContent("Email is not configured; share this link with dan@x.uz:");
    expect(screen.getByRole("textbox", { name: "Invitation link" })).toHaveValue("https://kyber.test/invite/abc");
    expect(calls.find((c) => c.key === "POST /api/v1/projects/KYB/invitations")?.body).toEqual({ email: "dan@x.uz", role: "member" });

    const list = await screen.findByRole("list", { name: "Pending invitations" });
    expect(within(list).getByText("dan@x.uz")).toBeInTheDocument();
    await userEvent.click(within(list).getByRole("button", { name: "Revoke invitation for dan@x.uz" }));
    expect(await screen.findByText("No pending invitations.")).toBeInTheDocument();
  });

  it("says when an existing user was added straight away", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/members": { status: 200, body: { items: [member] } },
      "GET /api/v1/projects/KYB/invitations": { status: 200, body: { items: [] } },
      "POST /api/v1/projects/KYB/invitations": { status: 200, body: { added: true } },
    });
    renderWithProviders(<Members projectKey="KYB" />);
    await userEvent.type(await screen.findByLabelText("Invite by email"), "timur@x.uz");
    await userEvent.click(screen.getByRole("button", { name: "Invite" }));
    expect(await screen.findByRole("status")).toHaveTextContent("timur@x.uz was added to the project.");
  });

  it("hides invitations from members who cannot manage them", async () => {
    mockApi({
      "GET /api/v1/projects/KYB/members": { status: 200, body: { items: [member] } },
      "GET /api/v1/projects/KYB/invitations": { status: 403, body: { code: "PROJECT_FORBIDDEN", title: "Your project role does not allow this." } },
    });
    renderWithProviders(<Members projectKey="KYB" />);
    expect(await screen.findByText("Aziza")).toBeInTheDocument();
    expect(screen.queryByRole("list", { name: "Pending invitations" })).not.toBeInTheDocument();
  });
});
