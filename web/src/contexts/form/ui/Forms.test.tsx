import { screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import type { Form } from "../application";
import { Forms } from "./Forms";

const form: Form = {
  id: "f-1", project_key: "KYB", name: "Bug report", intro: "Tell us what broke.", issue_type: "bug",
  questions: [{ id: "q1", label: "Steps", required: true, long: true }, { id: "q2", label: "Browser", required: false, long: false }], version: 1,
};
const problem = (status: number, detail: string) => ({ status, body: { type: "about:blank", title: "t", status, code: "X", detail } });

describe("forms", () => {
  it("designs a form, fills it in and opens the new issue", async () => {
    let items: Form[] = [];
    const calls = mockApi({
      "GET /api/v1/projects/KYB/forms": () => ({ status: 200, body: { items } }),
      "POST /api/v1/projects/KYB/forms": () => {
        items = [form];
        return { status: 201, body: form };
      },
      "POST /api/v1/forms/f-1/submissions": { status: 201, body: { issue_key: "KYB-7" } },
    });
    const onOpen = vi.fn();
    renderWithProviders(<Forms projectKey="KYB" onOpen={onOpen} />);
    expect(await screen.findByText(/No forms yet/)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "New form" }));
    await userEvent.type(screen.getByLabelText("Form name"), "Bug report");
    await userEvent.type(screen.getByLabelText("Intro"), "Tell us what broke.");
    await userEvent.selectOptions(screen.getByLabelText("Creates"), "bug");
    await userEvent.click(screen.getByRole("button", { name: "Add question" }));
    await userEvent.type(screen.getByLabelText("Question 1"), "Steps");
    await userEvent.click(screen.getAllByLabelText("Required")[0]!);
    await userEvent.click(screen.getAllByLabelText("Long answer")[0]!);
    await userEvent.click(screen.getByRole("button", { name: "Add question" }));
    await userEvent.type(screen.getByLabelText("Question 2"), "Browser");
    await userEvent.click(screen.getByRole("button", { name: "Add question" }));
    await userEvent.click(screen.getByRole("button", { name: "Remove question 3" }));
    await userEvent.click(screen.getByRole("button", { name: "Save form" }));
    expect(calls.find((c) => c.key === "POST /api/v1/projects/KYB/forms")?.body).toEqual({
      name: "Bug report", intro: "Tell us what broke.", issue_type: "bug",
      questions: [{ label: "Steps", required: true, long: true }, { label: "Browser", required: false, long: false }],
    });

    await userEvent.click(await screen.findByRole("button", { name: "Fill in Bug report" }));
    await userEvent.type(screen.getByLabelText("Summary"), "Login fails");
    await userEvent.click(screen.getByRole("button", { name: "Submit" }));
    expect(screen.getByRole("alert")).toHaveTextContent("Please answer: Steps");
    await userEvent.type(screen.getByLabelText("Steps *"), "Click login");
    await userEvent.type(screen.getByLabelText("Browser"), "Firefox");
    await userEvent.click(screen.getByRole("button", { name: "Submit" }));
    await waitFor(() =>
      expect(calls.find((c) => c.key === "POST /api/v1/forms/f-1/submissions")?.body).toEqual({ summary: "Login fails", answers: { q1: "Click login", q2: "Firefox" } }),
    );
    await userEvent.click(await screen.findByRole("button", { name: "KYB-7" }));
    expect(onOpen).toHaveBeenCalledWith("KYB-7");
  });

  it("edits, cancels and reports failures", async () => {
    const calls = mockApi({
      "GET /api/v1/projects/KYB/forms": { status: 200, body: { items: [form] } },
      "PUT /api/v1/forms/f-1": problem(403, "Only admins"),
      "DELETE /api/v1/forms/f-1": problem(403, "Only admins"),
      "POST /api/v1/forms/f-1/submissions": problem(422, "Bad answer"),
    });
    renderWithProviders(<Forms projectKey="KYB" onOpen={() => {}} />);
    await userEvent.click(await screen.findByRole("button", { name: "Edit Bug report" }));
    expect(screen.getByLabelText("Form name")).toHaveValue("Bug report");
    await userEvent.click(screen.getAllByLabelText("Required")[0]!);
    await userEvent.click(screen.getAllByLabelText("Long answer")[1]!);
    await userEvent.click(screen.getByRole("button", { name: "Save form" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Only admins");
    expect(calls.find((c) => c.key === "PUT /api/v1/forms/f-1")?.body).toMatchObject({ version: 1, questions: [{ required: false }, { long: true }] });
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));

    await userEvent.click(screen.getByRole("button", { name: "Fill in Bug report" }));
    await userEvent.type(screen.getByLabelText("Summary"), "x");
    await userEvent.type(screen.getByLabelText("Steps *"), "y");
    await userEvent.click(screen.getByRole("button", { name: "Submit" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Bad answer");
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));

    await userEvent.click(screen.getByRole("button", { name: "Delete Bug report" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Only admins");
  });

  it("shows a loading failure", async () => {
    mockApi({ "GET /api/v1/projects/KYB/forms": problem(404, "gone") });
    renderWithProviders(<Forms projectKey="KYB" onOpen={() => {}} />);
    expect(await screen.findByRole("alert")).toHaveTextContent("gone");
  });
});
