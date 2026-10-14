// Typed i18n: English text is the key; uz/ru must translate every key (a missing one fails tsc).
import { createContext, type ReactNode, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { en, type MessageKey } from "./en";
import { ru } from "./ru";
import { uz } from "./uz";

export const LANGUAGES = ["en", "uz", "ru"] as const;
export type Language = (typeof LANGUAGES)[number];
export type { MessageKey };

export const LANGUAGE_NAMES: Record<Language, string> = { en: "English", uz: "O‘zbekcha", ru: "Русский" };

const dictionaries: Record<Language, Record<MessageKey, string>> = { en, uz, ru };
const STORAGE_KEY = "kyber.lang";

export function isLanguage(v: unknown): v is Language {
  return typeof v === "string" && (LANGUAGES as readonly string[]).includes(v);
}

/** fromLocale maps a BCP 47 tag (navigator.language) to a supported language, falling back to en. */
export function fromLocale(tag: string | undefined): Language {
  const base = (tag ?? "").toLowerCase().split(/[-_]/)[0];
  return isLanguage(base) ? base : "en";
}

/** currentLanguage is the stored choice, else the browser's language. */
export function currentLanguage(): Language {
  try {
    const stored = localStorage.getItem(STORAGE_KEY);
    if (isLanguage(stored)) return stored;
  } catch {
    /* storage unavailable */
  }
  return fromLocale(typeof navigator === "undefined" ? undefined : navigator.language);
}

export function storeLanguage(lang: Language): void {
  try {
    localStorage.setItem(STORAGE_KEY, lang);
  } catch {
    /* storage unavailable */
  }
}

/** msg marks a message key kept in a constant and translated later with t(). */
export const msg = <K extends MessageKey>(key: K): K => key;

export type Vars =Record<string, string | number>;
export type Translate = (key: MessageKey, vars?: Vars) => string;

/** translate looks a key up and fills {name} placeholders. */
export function translate(lang: Language, key: MessageKey, vars?: Vars): string {
  const text = dictionaries[lang][key] ?? en[key];
  return vars ? text.replace(/\{(\w+)\}/g, (m, k: string) => (k in vars ? String(vars[k]) : m)) : text;
}

type I18n = { lang: Language; setLang: (l: Language) => void; t: Translate };

const I18nContext = createContext<I18n>({ lang: "en", setLang: () => {}, t: (k, v) => translate("en", k, v) });

export function I18nProvider({ children, initial }: { children: ReactNode; initial?: Language }) {
  const [lang, setLangState] = useState<Language>(() => initial ?? currentLanguage());
  useEffect(() => {
    document.documentElement.lang = lang;
  }, [lang]);
  const setLang = useCallback((l: Language) => {
    storeLanguage(l);
    setLangState(l);
  }, []);
  const value = useMemo<I18n>(() => ({ lang, setLang, t: (k, v) => translate(lang, k, v) }), [lang, setLang]);
  return <I18nContext.Provider value={value}>{children}</I18nContext.Provider>;
}

export function useI18n(): I18n {
  return useContext(I18nContext);
}

export function useT(): Translate {
  return useContext(I18nContext).t;
}
