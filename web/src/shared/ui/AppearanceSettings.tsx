import { type ChangeEvent, useState } from "react";
import {
  type Appearance,
  applyAppearance,
  loadAppearance,
  MAX_CUSTOM_WALLPAPER,
  saveAppearance,
  type Theme,
  THEMES,
  type Wallpaper,
  WALLPAPERS,
} from "../appearance";
import { useT } from "../i18n/i18n";

/** Theme names stay in their own words in every language, like product names. */
const THEME_NAMES: Record<Theme, string> = {
  system: "System",
  light: "Light",
  dark: "Dark",
  midnight: "Midnight",
  ocean: "Ocean",
  forest: "Forest",
  sunset: "Sunset",
  rose: "Rose",
  nord: "Nord",
  dracula: "Dracula",
  contrast: "High contrast",
};

/** Preview colours: background, surface, accent. */
const THEME_PREVIEW: Record<Theme, [string, string, string]> = {
  system: ["#f4f7fb", "#0a111c", "#0b6fd8"],
  light: ["#f4f7fb", "#ffffff", "#0b6fd8"],
  dark: ["#0a111c", "#111b2a", "#3fb6ff"],
  midnight: ["#05060f", "#0d1020", "#8b7bff"],
  ocean: ["#e9f6fb", "#ffffff", "#00879e"],
  forest: ["#0f1a14", "#16251c", "#5fcf80"],
  sunset: ["#fff5ee", "#ffffff", "#e8590c"],
  rose: ["#fdf2f6", "#ffffff", "#d6336c"],
  nord: ["#2e3440", "#3b4252", "#88c0d0"],
  dracula: ["#1e1f29", "#282a36", "#bd93f9"],
  contrast: ["#000000", "#000000", "#ffd400"],
};

const WALLPAPER_NAMES: Record<Wallpaper, string> = {
  none: "None",
  aurora: "Aurora",
  dusk: "Dusk",
  sea: "Sea",
  meadow: "Meadow",
  peach: "Peach",
  mesh: "Mesh",
  grid: "Grid",
  dots: "Dots",
  waves: "Waves",
  paper: "Paper",
  custom: "My image",
};

/** AppearanceSettings: colour theme and wallpaper for this browser. */
export function AppearanceSettings() {
  const t = useT();
  const [a, setA] = useState<Appearance>(loadAppearance);
  const [error, setError] = useState("");

  function update(next: Appearance) {
    setA(next);
    saveAppearance(next);
    applyAppearance(next);
  }

  function upload(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    if (!file.type.startsWith("image/") || file.size > MAX_CUSTOM_WALLPAPER) {
      setError(t("Choose an image of at most 1 MB."));
      return;
    }
    setError("");
    const reader = new FileReader();
    reader.onload = () => {
      if (typeof reader.result === "string") update({ ...a, wallpaper: "custom", custom: reader.result });
    };
    reader.readAsDataURL(file);
  }

  return (
    <section className="settings panel" aria-label={t("Appearance")}>
      <h2>{t("Appearance")}</h2>
      <h3>{t("Theme")}</h3>
      <div className="swatches" role="group" aria-label={t("Theme")}>
        {THEMES.map((th) => {
          const [bg, surface, accent] = THEME_PREVIEW[th];
          return (
            <button key={th} type="button" className="swatch" aria-pressed={a.theme === th} onClick={() => update({ ...a, theme: th })}>
              <span className="swatch-preview" style={{ background: `linear-gradient(90deg, ${bg} 0 45%, ${surface} 45% 80%, ${accent} 80%)` }} />
              {THEME_NAMES[th]}
            </button>
          );
        })}
      </div>
      <h3>{t("Wallpaper")}</h3>
      <div className="swatches" role="group" aria-label={t("Wallpaper")}>
        {WALLPAPERS.filter((w) => w !== "custom" || a.custom).map((w) => (
          <button key={w} type="button" className="swatch" aria-pressed={a.wallpaper === w} onClick={() => update({ ...a, wallpaper: w })}>
            <span
              className={`swatch-preview wp-${w}`}
              style={w === "custom" ? { backgroundImage: `url("${a.custom}")` } : undefined}
            />
            {WALLPAPER_NAMES[w]}
          </button>
        ))}
      </div>
      <label className="ghost button">
        {t("Upload image…")}
        <input type="file" accept="image/*" hidden aria-label={t("Upload wallpaper")} onChange={upload} />
      </label>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
    </section>
  );
}
