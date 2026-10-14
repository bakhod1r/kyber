import { screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { Huddle } from "./Huddle";

const names = new Map([["u-1", "Ali"], ["u-2", "Bobur"]]);
const person = (user_id: string) => ({ user_id, joined_at: "", audio: true, video: false, screen: false });
const live = (...ids: string[]) => ({ call: { id: "k-1", conversation_id: "c-1", started_by: ids[0], started_at: "", live: true, participants: ids.map(person) } });

describe("Huddle (S57)", () => {
  it("offers to start a huddle when none is live", async () => {
    mockApi({ "GET /api/v1/conversations/c-1/call": { status: 200, body: { call: null } } });
    renderWithProviders(<Huddle conversationId="c-1" me="u-1" names={names} />);
    expect(await screen.findByRole("button", { name: "Start huddle" })).toBeEnabled();
  });

  it("shows who is in the live huddle and lets me join", async () => {
    mockApi({ "GET /api/v1/conversations/c-1/call": { status: 200, body: live("u-2") } });
    renderWithProviders(<Huddle conversationId="c-1" me="u-1" names={names} />);
    expect(await screen.findByRole("button", { name: "Join huddle (1)" })).toBeEnabled();
    expect(screen.getByText(/Huddle: Bobur/)).toBeInTheDocument();
  });

  it("cannot join a full huddle", async () => {
    mockApi({ "GET /api/v1/conversations/c-1/call": { status: 200, body: live("a", "b", "c", "d", "e", "f") } });
    renderWithProviders(<Huddle conversationId="c-1" me="u-1" names={names} />);
    expect(await screen.findByRole("button", { name: "Join huddle (6)" })).toBeDisabled();
  });

  it("a room call takes more than six people (S99)", async () => {
    const six = live("a", "b", "c", "d", "e", "f");
    mockApi({ "GET /api/v1/conversations/c-1/call": { status: 200, body: { call: { ...six.call, room: true } } } });
    renderWithProviders(<Huddle conversationId="c-1" me="u-1" names={names} />);
    expect(await screen.findByRole("button", { name: "Join huddle (6)" })).toBeEnabled();
  });
});
