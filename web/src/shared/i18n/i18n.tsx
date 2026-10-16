// Typed i18n: English text is the key; uz/ru must translate every key (a missing one fails tsc); 22
// more languages translate the core interface and fall back to English.
import { createContext, type ReactNode, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { en, type MessageKey } from "./en";
import { setDateLanguage } from "./labels";
import { ru } from "./ru";
import { uz } from "./uz";
import { zh } from "./zh";
import { hi } from "./hi";
import { es } from "./es";
import { ar } from "./ar";
import { fr } from "./fr";
import { bn } from "./bn";
import { pt } from "./pt";
import { ur } from "./ur";
import { id } from "./id";
import { de } from "./de";
import { ja } from "./ja";
import { tr } from "./tr";
import { ko } from "./ko";
import { vi } from "./vi";
import { it } from "./it";
import { fa } from "./fa";
import { pl } from "./pl";
import { uk } from "./uk";
import { nl } from "./nl";
import { th } from "./th";
import { kk } from "./kk";
import { az } from "./az";

export const LANGUAGES = ["en", "uz", "ru", "zh", "hi", "es", "ar", "fr", "bn", "pt", "ur", "id", "de", "ja", "tr", "ko", "vi", "it", "fa", "pl", "uk", "nl", "th", "kk", "az"] as const;
export type Language = (typeof LANGUAGES)[number];
export type { MessageKey };

export const LANGUAGE_NAMES: Record<Language, string> = { en: "English", uz: "O‘zbekcha", ru: "Русский", zh: "中文", hi: "हिन्दी", es: "Español", ar: "العربية", fr: "Français", bn: "বাংলা", pt: "Português", ur: "اردو", id: "Bahasa Indonesia", de: "Deutsch", ja: "日本語", tr: "Türkçe", ko: "한국어", vi: "Tiếng Việt", it: "Italiano", fa: "فارسی", pl: "Polski", uk: "Українська", nl: "Nederlands", th: "ไทย", kk: "Қазақша", az: "Azərbaycanca" };

/** Languages written right to left. */
export const RTL: readonly Language[] = ["ar", "fa", "ur"];

// en, uz and ru translate every key (checked by tsc); the others cover the core interface and fall
// back to English for the rest.
const dictionaries: Record<Language, Partial<Record<MessageKey, string>>> = { en, uz, ru, zh, hi, es, ar, fr, bn, pt, ur, id, de, ja, tr, ko, vi, it, fa, pl, uk, nl, th, kk, az };
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

/** translateLabel translates text that is a known message (a default status or column name) and returns
 * anything else — a name someone typed — unchanged. */
export function translateLabel(lang: Language, text: string): string {
  return text in en ? translate(lang, text as MessageKey) : text;
}

type I18n = { lang: Language; setLang: (l: Language) => void; t: Translate };

const I18nContext = createContext<I18n>({ lang: "en", setLang: () => {}, t: (k, v) => translate("en", k, v) });

export function I18nProvider({ children, initial }: { children: ReactNode; initial?: Language }) {
  const [lang, setLangState] = useState<Language>(() => initial ?? currentLanguage());
  setDateLanguage(lang); // before children render, so dates follow the language at once
  useEffect(() => {
    document.documentElement.lang = lang;
    document.documentElement.dir = RTL.includes(lang) ? "rtl" : "ltr";
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

/** useLabel returns translateLabel bound to the current language. */
export function useLabel(): (text: string) => string {
  const { lang } = useContext(I18nContext);
  return useCallback((text: string) => translateLabel(lang, text), [lang]);
}
