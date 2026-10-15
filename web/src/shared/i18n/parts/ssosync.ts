// SSO group mapping strings (S106). English is the key; uz and ru must translate every key (a missing one fails tsc).
export const en = {
  "SSO group mappings": "SSO group mappings",
  "Members of an identity-provider group get the mapped role at every sign-in.": "Members of an identity-provider group get the mapped role at every sign-in.",
  "No group mappings yet.": "No group mappings yet.",
  "Remove mapping {group}": "Remove mapping {group}",
  "Workspace": "Workspace",
  "Project": "Project",
  "Project key": "Project key",
  "Scope": "Scope",
  "Mapped role": "Mapped role",
  "Add mapping": "Add mapping",
};

export const uz: Record<keyof typeof en, string> = {
  "SSO group mappings": "SSO guruhlarini rollarga bog‘lash",
  "Members of an identity-provider group get the mapped role at every sign-in.": "Identifikatsiya provayderi guruhi a’zolari har kirishda bog‘langan rolni oladi.",
  "No group mappings yet.": "Hozircha guruh bog‘lanmalari yo‘q.",
  "Remove mapping {group}": "{group} bog‘lanmasini o‘chirish",
  "Workspace": "Ish maydoni",
  "Project": "Loyiha",
  "Project key": "Loyiha kaliti",
  "Scope": "Doira",
  "Mapped role": "Bog‘langan rol",
  "Add mapping": "Bog‘lanma qo‘shish",
};

export const ru: Record<keyof typeof en, string> = {
  "SSO group mappings": "Сопоставление групп SSO",
  "Members of an identity-provider group get the mapped role at every sign-in.": "Участники группы поставщика удостоверений получают сопоставленную роль при каждом входе.",
  "No group mappings yet.": "Сопоставлений групп пока нет.",
  "Remove mapping {group}": "Удалить сопоставление {group}",
  "Workspace": "Рабочее пространство",
  "Project": "Проект",
  "Project key": "Ключ проекта",
  "Scope": "Область",
  "Mapped role": "Сопоставленная роль",
  "Add mapping": "Добавить сопоставление",
};
