import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Route, Routes } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { InvitePage } from "./InvitePage";

const preview = { project: "KYB", project_name: "Kyber", invited_by: "Aziza", email: "dan@x.uz", role: "member", expires_at: "2026-10-22T09:00:00Z", expired: false };

function renderAt(signedIn: boolean) {
  return renderWithProviders(
    <Routes>
      <Route path="/invite/:token" element={<InvitePage signedIn={signedIn} />} />
      <Route path="/projects/:key" element={<p>project page</p>} />
    </Routes>,
    "/invite/abc",
  );
}

describe("InvitePage (B1)", () => {
  it("shows the invitation and joins the project", async () => {
    const calls = mockApi({
      "GET /api/v1/invitations/abc": { status: 200, body: preview },
      "POST /api/v1/invitations/abc/accept": { status: 200, body: { project: "KYB" } },
    });
    renderAt(true);
    expect(await screen.findByRole("heading", { name: "Join Kyber" })).toBeInTheDocument();
    expect(screen.getByText(/Aziza invited dan@x.uz/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Accept invitation" }));
    expect(await screen.findByText("project page")).toBeInTheDocument();
    expect(calls.some((c) => c.key === "POST /api/v1/invitations/abc/accept")).toBe(true);
  });

  it("asks a visitor to sign in or sign up first and comes back", async () => {
    mockApi({ "GET /api/v1/invitations/abc": { status: 200, body: preview } });
    renderAt(false);
    expect(await screen.findByRole("link", { name: "Create an account" })).toHaveAttribute("href", "/signup?next=%2Finvite%2Fabc");
    expect(screen.getByRole("link", { name: "Log in" })).toHaveAttribute("href", "/login?next=%2Finvite%2Fabc");
    expect(screen.queryByRole("button", { name: "Accept invitation" })).not.toBeInTheDocument();
  });

  it("explains a wrong account, an expired and an unknown link", async () => {
    mockApi({
      "GET /api/v1/invitations/abc": { status: 200, body: preview },
      "POST /api/v1/invitations/abc/accept": { status: 403, body: { code: "INVITATION_OTHER_EMAIL", title: "This invitation was sent to another email address. Sign in with that address." } },
    });
    renderAt(true);
    await userEvent.click(await screen.findByRole("button", { name: "Accept invitation" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("another email address");
  });

  it("shows an expired invitation without the accept button", async () => {
    mockApi({ "GET /api/v1/invitations/abc": { status: 200, body: { ...preview, expired: true } } });
    renderAt(true);
    expect(await screen.findByText("This invitation has expired. Ask for a new one.")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Accept invitation" })).not.toBeInTheDocument();
  });

  it("says when the link does not exist", async () => {
    mockApi({ "GET /api/v1/invitations/abc": { status: 404, body: { code: "INVITATION_NOT_FOUND", title: "This invitation does not exist or was cancelled." } } });
    renderAt(true);
    expect(await screen.findByRole("alert")).toHaveTextContent("does not exist or was cancelled");
  });
});
