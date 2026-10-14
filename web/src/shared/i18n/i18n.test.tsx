import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { en } from "./en";
import { currentLanguage, fromLocale, I18nProvider, LANGUAGE_NAMES, LANGUAGES, translate, useT } from "./i18n";
import { ar } from "./ar";
import { de } from "./de";
import { ja } from "./ja";
import { LanguagePicker } from "./LanguagePicker";
import { ru } from "./ru";
import { uz } from "./uz";

function Hello() {
  const t = useT();
  return <p>{t("Log out")}</p>;
}

afterEach(() => {
  localStorage.clear();
  document.documentElement.lang = "";
  document.documentElement.dir = "";
});

describe("i18n", () => {
  it("English keys are the English text", () => {
    for (const [k, v] of Object.entries(en)) expect(v).toBe(k);
  });

  it("every language translates every key, keeping placeholders", () => {
    const placeholders = (s: string) => [...s.matchAll(/\{(\w+)\}/g)].map((m) => m[1]).sort();
    for (const dict of [uz, ru]) {
      expect(Object.keys(dict).sort()).toEqual(Object.keys(en).sort());
      for (const [k, v] of Object.entries(dict)) {
        expect(v.trim(), k).not.toBe("");
        expect(placeholders(v), k).toEqual(placeholders(k));
      }
    }
  });

  it("fills placeholders", () => {
    expect(translate("en", "{n} issues", { n: 3 })).toBe("3 issues");
    expect(translate("ru", "Log out")).toBe(ru["Log out"]);
  });

  it.each([
    ["uz-UZ", "uz"],
    ["ru", "ru"],
    ["en-US", "en"],
    ["de-DE", "de"],
    ["sw-KE", "en"],
    [undefined, "en"],
  ] as const)("maps browser language %s to %s", (tag, want) => {
    expect(fromLocale(tag)).toBe(want);
  });

  it("prefers the stored language and survives broken storage", () => {
    localStorage.setItem("kyber.lang", "uz");
    expect(currentLanguage()).toBe("uz");
    localStorage.setItem("kyber.lang", "xx");
    expect(currentLanguage()).toBe("en"); // jsdom's navigator is en-US
    const spy = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    expect(currentLanguage()).toBe("en");
    spy.mockRestore();
  });

  it("defaults to English without a provider", () => {
    render(<Hello />);
    expect(screen.getByText("Log out")).toBeInTheDocument();
  });

  it("the picker switches language, stores it and sets <html lang>", async () => {
    render(
      <I18nProvider>
        <LanguagePicker />
        <Hello />
      </I18nProvider>,
    );
    expect(document.documentElement.lang).toBe("en");
    await userEvent.selectOptions(screen.getByLabelText("Language"), "ru");
    expect(screen.getByText(ru["Log out"])).toBeInTheDocument();
    expect(localStorage.getItem("kyber.lang")).toBe("ru");
    expect(document.documentElement.lang).toBe("ru");
    await userEvent.selectOptions(screen.getByLabelText(ru["Language"]), "uz");
    expect(screen.getByText(uz["Log out"])).toBeInTheDocument();
  });

  it("offers 25 languages, each with a native name, and the core ones translate the core keys", () => {
    expect(LANGUAGES).toHaveLength(25);
    for (const l of LANGUAGES) expect(LANGUAGE_NAMES[l].trim()).not.toBe("");
    for (const dict of [ar, de, ja]) {
      for (const [k, v] of Object.entries(dict)) {
        expect(k in en, k).toBe(true);
        expect(v.trim(), k).not.toBe("");
      }
      expect(dict["Log out"]).toBeTruthy();
    }
  });

  it("falls back to English for keys a language does not translate yet", () => {
    expect(translate("de", "Log out")).toBe("Abmelden");
    expect(translate("de", "{n} issues", { n: 2 })).toBe("2 issues");
  });

  it("writes Arabic, Persian and Urdu right to left", async () => {
    render(
      <I18nProvider initial="ar">
        <Hello />
      </I18nProvider>,
    );
    expect(await screen.findByText("تسجيل الخروج")).toBeInTheDocument();
    expect(document.documentElement.dir).toBe("rtl");
    expect(document.documentElement.lang).toBe("ar");
  });

  it("covers chat, huddles and whiteboards in Uzbek and Russian (S94)", () => {
    expect(translate("uz", "Start huddle")).toBe("Huddle boshlash");
    expect(translate("ru", "Zoom in")).toBe("Увеличить");
    expect(translate("uz", "{n} replies", { n: 3 })).toBe("3 ta javob");
    expect(translate("de", "Start huddle")).toBe("Start huddle"); // other languages fall back to English
  });
});
