import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { mockApi, renderWithProviders } from "../../../shared/test/test-utils";
import { ArchiveControl } from "./ArchiveControl";

const project = (archived: boolean) => ({ id: "p-1", key: "KYB", name: "Kyber", archived });

describe("ArchiveControl (B10)", () => {
  it("lets an admin archive after confirming, then shows the read-only banner with Restore", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(true);
    let archived = false;
    const calls = mockApi({
      "GET /api/v1/projects": () => ({ status: 200, body: { items: [project(archived)] } }),
      "POST /api/v1/projects/KYB/archive": () => {
        archived = true;
        return { status: 200, body: project(true) };
      },
      "POST /api/v1/projects/KYB/restore": () => {
        archived = false;
        return { status: 200, body: project(false) };
      },
    });
    renderWithProviders(<ArchiveControl projectKey="KYB" admin />);
    await userEvent.click(await screen.findByRole("button", { name: "Archive project" }));
    expect(window.confirm).toHaveBeenCalled();
    expect(await screen.findByRole("status")).toHaveTextContent("This project is archived and read-only.");
    await userEvent.click(screen.getByRole("button", { name: "Restore project" }));
    expect(await screen.findByRole("button", { name: "Archive project" })).toBeInTheDocument();
    expect(calls.map((c) => c.key)).toContain("POST /api/v1/projects/KYB/restore");
  });

  it("shows members the banner without the buttons", async () => {
    mockApi({ "GET /api/v1/projects": { status: 200, body: { items: [project(true)] } } });
    renderWithProviders(<ArchiveControl projectKey="KYB" admin={false} />);
    expect(await screen.findByRole("status")).toHaveTextContent("archived");
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("does nothing when the admin cancels", async () => {
    vi.spyOn(window, "confirm").mockReturnValue(false);
    const calls = mockApi({ "GET /api/v1/projects": { status: 200, body: { items: [project(false)] } } });
    renderWithProviders(<ArchiveControl projectKey="KYB" admin />);
    await userEvent.click(await screen.findByRole("button", { name: "Archive project" }));
    expect(calls.some((c) => c.key.startsWith("POST"))).toBe(false);
  });
});
