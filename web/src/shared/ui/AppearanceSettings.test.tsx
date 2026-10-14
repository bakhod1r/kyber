import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";
import { applyAppearance, DEFAULT_APPEARANCE, loadAppearance } from "../appearance";
import { AppearanceSettings } from "./AppearanceSettings";

afterEach(() => {
  localStorage.clear();
  applyAppearance(DEFAULT_APPEARANCE);
});

describe("AppearanceSettings", () => {
  it("switches theme and wallpaper and remembers them", async () => {
    render(<AppearanceSettings />);
    await userEvent.click(screen.getByRole("button", { name: "Nord" }));
    expect(document.documentElement.dataset.theme).toBe("nord");
    expect(screen.getByRole("button", { name: "Nord" })).toHaveAttribute("aria-pressed", "true");
    await userEvent.click(screen.getByRole("button", { name: "Aurora" }));
    expect(document.documentElement.dataset.wallpaper).toBe("aurora");
    expect(loadAppearance()).toEqual({ theme: "nord", wallpaper: "aurora", custom: "" });
  });

  it("uploads a small image as a custom wallpaper and refuses large ones", async () => {
    render(<AppearanceSettings />);
    const input = screen.getByLabelText("Upload wallpaper");
    const big = new File(["x"], "big.png", { type: "image/png" });
    Object.defineProperty(big, "size", { value: 2 * 1024 * 1024 });
    await userEvent.upload(input, big);
    expect(screen.getByRole("alert")).toHaveTextContent("at most 1 MB");
    await userEvent.upload(input, new File([new Uint8Array([137, 80, 78, 71])], "me.png", { type: "image/png" }));
    await waitFor(() => expect(document.documentElement.dataset.wallpaper).toBe("custom"));
    expect(screen.getByRole("button", { name: "My image" })).toHaveAttribute("aria-pressed", "true");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
});
