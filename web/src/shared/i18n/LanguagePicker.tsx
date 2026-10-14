import { LANGUAGE_NAMES, LANGUAGES, isLanguage, useI18n } from "./i18n";

export function LanguagePicker() {
  const { lang, setLang, t } = useI18n();
  return (
    <select
      aria-label={t("Language")}
      className="lang-picker"
      value={lang}
      onChange={(e) => {
        if (isLanguage(e.target.value)) setLang(e.target.value);
      }}
    >
      {LANGUAGES.map((l) => (
        <option key={l} value={l}>
          {LANGUAGE_NAMES[l]}
        </option>
      ))}
    </select>
  );
}
