// Appearance: colour theme and wallpaper, chosen per browser (localStorage) and applied as attributes
// on <html> that styles.css turns into colour tokens and a page background.

export const THEMES = ["system", "light", "dark", "midnight", "ocean", "forest", "sunset", "rose", "nord", "dracula", "contrast"] as const;
export type Theme = (typeof THEMES)[number];

export const WALLPAPERS = ["none", "aurora", "dusk", "sea", "meadow", "peach", "mesh", "grid", "dots", "waves", "paper", "custom"] as const;
export type Wallpaper = (typeof WALLPAPERS)[number];

export type Appearance = { theme: Theme; wallpaper: Wallpaper; custom: string };
export const DEFAULT_APPEARANCE: Appearance = { theme: "system", wallpaper: "none", custom: "" };

/** A custom wallpaper is stored inline (data URL), so it is kept small. */
export const MAX_CUSTOM_WALLPAPER = 1024 * 1024;

const KEY = "kyber.appearance";

const isTheme = (v: unknown): v is Theme => typeof v === "string" && (THEMES as readonly string[]).includes(v);
const isWallpaper = (v: unknown): v is Wallpaper => typeof v === "string" && (WALLPAPERS as readonly string[]).includes(v);

/** parseAppearance reads a stored value, ignoring anything unknown or malformed. */
export function parseAppearance(raw: string | null): Appearance {
  try {
    const v: unknown = raw ? JSON.parse(raw) : null;
    if (typeof v !== "object" || v === null) return DEFAULT_APPEARANCE;
    const o = v as Record<string, unknown>;
    const custom = typeof o.custom === "string" && o.custom.startsWith("data:image/") ? o.custom : "";
    const wallpaper = isWallpaper(o.wallpaper) && (o.wallpaper !== "custom" || custom) ? o.wallpaper : "none";
    return { theme: isTheme(o.theme) ? o.theme : "system", wallpaper, custom };
  } catch {
    return DEFAULT_APPEARANCE;
  }
}

export function loadAppearance(): Appearance {
  try {
    return parseAppearance(localStorage.getItem(KEY));
  } catch {
    return DEFAULT_APPEARANCE;
  }
}

export function saveAppearance(a: Appearance): void {
  try {
    localStorage.setItem(KEY, JSON.stringify(a));
  } catch {
    /* storage unavailable or full: the choice lasts until reload */
  }
}

/** applyAppearance sets data-theme / data-wallpaper on the root element ("system" and "none" clear them). */
export function applyAppearance(a: Appearance, root: HTMLElement = document.documentElement): void {
  if (a.theme === "system") delete root.dataset.theme;
  else root.dataset.theme = a.theme;
  if (a.wallpaper === "none") delete root.dataset.wallpaper;
  else root.dataset.wallpaper = a.wallpaper;
  if (a.wallpaper === "custom" && a.custom) root.style.setProperty("--wallpaper-custom", `url("${a.custom}")`);
  else root.style.removeProperty("--wallpaper-custom");
}
