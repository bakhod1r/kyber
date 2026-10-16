// Display labels for ids the API sends (issue types, priorities, roles, sprint states) and locale-aware
// dates in the language the user picked (not the browser's).
import { en } from "./en";
import type { Language, MessageKey, Translate } from "./i18n";

const isKey = (s: string): s is MessageKey => Object.prototype.hasOwnProperty.call(en, s);

/** label translates text that is a message key and returns anything else unchanged (user data). */
export const label = (t: Translate, s: string): string => (isKey(s) ? t(s) : s);

const TYPES: Record<string, MessageKey> = { epic: "Epic", story: "Story", task: "Task", bug: "Bug", subtask: "Sub-task" };
const PRIORITIES: Record<string, MessageKey> = { highest: "Highest", high: "High", medium: "Medium", low: "Low", lowest: "Lowest" };
const ROLES: Record<string, MessageKey> = { admin: "Admin", member: "Member", viewer: "Viewer" };
const SPRINT_STATES: Record<string, MessageKey> = { active: "Active", planned: "Planned", closed: "Closed" };

const from = (table: Record<string, MessageKey>) => (t: Translate, id: string) => {
  const k = table[id];
  return k ? t(k) : id;
};
export const typeLabel = from(TYPES);
export const priorityLabel = from(PRIORITIES);
export const roleLabel = from(ROLES);
export const sprintStateLabel = from(SPRINT_STATES);

/** permissionLabel turns BROWSE_PROJECTS into "Browse projects", translated. */
export const permissionLabel = (t: Translate, id: string) => label(t, id.charAt(0) + id.slice(1).toLowerCase().replaceAll("_", " "));

// Dates follow the chosen language; the provider keeps this in step with it.
const LOCALES: Partial<Record<Language, string>> = { uz: "uz-Latn-UZ", zh: "zh-CN", pt: "pt-BR" };
let locale = "en";
export function setDateLanguage(lang: Language): void {
  locale = LOCALES[lang] ?? lang;
}
const fmt = (iso: string | number | Date, o: Intl.DateTimeFormatOptions) => {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "" : d.toLocaleString(locale, o);
};
/** formatDateTime: "15 Oct 2026, 09:30" in the user's language. */
export const formatDateTime = (iso: string | number | Date) => fmt(iso, { dateStyle: "medium", timeStyle: "short" });
/** formatDate: "15 Oct 2026". */
export const formatDate = (iso: string | number | Date) => fmt(iso, { dateStyle: "medium" });
/** formatShortDate: "15 Oct" (UTC, for chart axes). */
export const formatShortDate = (iso: string | number | Date) => fmt(iso, { month: "short", day: "numeric", timeZone: "UTC" });
