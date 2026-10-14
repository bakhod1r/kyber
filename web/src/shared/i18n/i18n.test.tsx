import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { en } from "./en";
import { currentLanguage, fromLocale, I18nProvider, translate, useT } from "./i18n";
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
    ["de-DE", "en"],
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
});
