import { afterEach, describe, expect, it } from "vitest";
import { applyAppearance, DEFAULT_APPEARANCE, loadAppearance, parseAppearance, saveAppearance, THEMES, WALLPAPERS } from "./appearance";

afterEach(() => {
  localStorage.clear();
  applyAppearance(DEFAULT_APPEARANCE);
});

describe("appearance", () => {
  it("offers ten themes besides the system one and ten wallpapers besides none and a custom image", () => {
    expect(THEMES.filter((t) => t !== "system")).toHaveLength(10);
    expect(WALLPAPERS.filter((w) => w !== "none" && w !== "custom")).toHaveLength(10);
  });

  it("parses stored choices and rejects anything unknown", () => {
    expect(parseAppearance(null)).toEqual(DEFAULT_APPEARANCE);
    expect(parseAppearance("not json")).toEqual(DEFAULT_APPEARANCE);
    expect(parseAppearance("42")).toEqual(DEFAULT_APPEARANCE);
    expect(parseAppearance(JSON.stringify({ theme: "nord", wallpaper: "grid" }))).toEqual({ theme: "nord", wallpaper: "grid", custom: "" });
    expect(parseAppearance(JSON.stringify({ theme: "neon", wallpaper: "lava" }))).toEqual(DEFAULT_APPEARANCE);
    // A custom wallpaper needs an image data URL; anything else (e.g. a remote URL) is dropped.
    expect(parseAppearance(JSON.stringify({ wallpaper: "custom", custom: "https://evil.example/x.png" })).wallpaper).toBe("none");
    expect(parseAppearance(JSON.stringify({ wallpaper: "custom", custom: "data:image/png;base64,AA" })).wallpaper).toBe("custom");
  });

  it("saves, loads and applies to the root element", () => {
    saveAppearance({ theme: "dracula", wallpaper: "custom", custom: "data:image/png;base64,AA" });
    const a = loadAppearance();
    applyAppearance(a);
    const root = document.documentElement;
    expect(root.dataset.theme).toBe("dracula");
    expect(root.dataset.wallpaper).toBe("custom");
    expect(root.style.getPropertyValue("--wallpaper-custom")).toContain("data:image/png");
    applyAppearance(DEFAULT_APPEARANCE);
    expect(root.dataset.theme).toBeUndefined();
    expect(root.dataset.wallpaper).toBeUndefined();
    expect(root.style.getPropertyValue("--wallpaper-custom")).toBe("");
  });
});
