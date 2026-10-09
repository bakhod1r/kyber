# Kyber — Master Plan

> Jira'ning asosiy (core) funksionalligiga ega, self-hosted, open-source issue & project tracker.
> Stack: **Go** (backend) · **React + TypeScript** (frontend) · **PostgreSQL** · **Redis** (ixtiyoriy).
> Ishlab chiqish jarayoni: [`awesome-agents`](https://github.com/bakhod1r/awesome-agents) zanjiri —
> *architect decides → engineer implements → quality verifies → release ships.*

---

## 1. Maqsad va chegaralar

### 1.1 Vizyon
Kichik va o'rta jamoalar uchun **tez, yengil, bitta binary bilan ishga tushadigan** Jira muqobili.
`docker compose up` — 2 daqiqada ishlaydigan tracker.

### 1.2 Muvaffaqiyat mezonlari (o'lchanadigan)
| Metrika | Maqsad (v1.0) |
|---|---|
| Sovuq start (docker compose) | < 2 daqiqa |
| API p95 latency (10k issue, 50 RPS) | < 100 ms |
| Board sahifasi yuklanishi (LCP) | < 1.5 s |
| Backend test coverage (domain + service) | ≥ 80% |
| Server RAM (idle) | < 150 MB |
| GitHub stars (6 oy) | 1 000+ |

### 1.3 Scope'ga kirmaydi (v1.0)
Confluence analogi, marketplace/plugin SDK, mobil ilova, Service Desk, Advanced Roadmaps / Portfolio.

---

## 2. Core funksionallik (Jira paritet xaritasi)

| # | Modul | v0.1 MVP | v0.5 Beta | v1.0 |
|---|---|---|---|---|
| F1 | **Auth & foydalanuvchilar** — email/parol, sessiya/JWT, profil | ✅ | OIDC/OAuth (Google, GitHub) | SAML, SCIM |
| F2 | **Workspace / Organization** — multi-tenant, a'zolar, taklif | ✅ | | |
| F3 | **Project** — key (`KYB`), avatar, a'zolar, rollar | ✅ | arxivlash | shablonlar |
| F4 | **Issue** — type (Epic/Story/Task/Bug/Sub-task), title, Markdown description, priority, assignee, reporter, labels, due date, `KYB-123` kalit | ✅ | custom fields | |
| F5 | **Workflow** — status'lar, o'tishlar (transitions), status kategoriyalari (To Do / In Progress / Done) | default workflow | **workflow editor** | transition conditions/validators |
| F6 | **Board** — Kanban, drag & drop, ustun = status, WIP limit | ✅ Kanban | Scrum board | swimlanes, quick filters |
| F7 | **Backlog & Sprint** — sprint yaratish/boshlash/yopish, rank (LexoRank) | | ✅ | velocity |
| F8 | **Comments & mentions** — `@user`, Markdown, tahrir tarixi | ✅ | reactions | |
| F9 | **Activity / History** — har bir o'zgarish audit log | ✅ | | |
| F10 | **Issue links** — blocks / relates / duplicates, Epic → child | parent/sub-task | ✅ link turlari | |
| F11 | **Search** — oddiy filter | ✅ | **KQL** (JQL analogi) parser | saved filters, full-text (pg_trgm / tsvector) |
| F12 | **Notifications** — in-app, email | in-app | email + watchers | digest |
| F13 | **Attachments** — S3/MinIO/local | | ✅ | preview |
| F14 | **Permissions** — RBAC: Admin / Member / Viewer per project | ✅ basic | permission schemes | issue-level security |
| F15 | **Realtime** — board/issue live yangilanish (WebSocket/SSE) | | ✅ | presence |
| F16 | **Reports** — burndown, CFD, sprint report | | burndown | CFD, control chart |
| F17 | **Integrations** — webhooks, REST API tokens, GitHub commit/PR linking | API tokens | webhooks | GitHub app |
| F18 | **Import** — Jira CSV/JSON import | | | ✅ |
| F19 | **i18n** — en, uz, ru | en | uz, ru | |

---

## 3. Arxitektura

### 3.1 Uslub
**Modular monolith** (bitta Go binary), ichida aniq bounded context'lar. Mikroservislar — keraksiz
murakkablik; modullar orasidagi chegara keyinchalik ajratish imkonini saqlaydi.

```
               ┌────────────────────────── kyber (single Go binary) ──────────────────────────┐
 React SPA ──► │  HTTP (chi) ─ OpenAPI handlers ─┐                                            │
 (embed.FS)    │  WebSocket/SSE hub ─────────────┤                                            │
               │                                 ▼                                            │
               │   identity │ workspace │ project │ issue │ workflow │ agile │ search │ notify │
               │        (har biri: domain → service → repository)                             │
               │                                 │            ▲                               │
               │           outbox → event bus (in-proc) ──────┘ → webhooks, email, realtime   │
               └─────────────────────────────────┼────────────────────────────────────────────┘
                                                 ▼
                                    PostgreSQL 16  (+ Redis: cache/pubsub, ixtiyoriy)
                                    S3 / MinIO / local FS (attachments)
```

### 3.2 Backend (Go)
| Qatlam | Tanlov | Sabab |
|---|---|---|
| Go versiyasi | 1.23+ | generics, `log/slog`, `net/http` routing |
| Router | `go-chi/chi` v5 | stdlib-mos, middleware ekotizimi |
| API kontrakt | OpenAPI 3.1 → `oapi-codegen` (server) + `openapi-typescript` (client) | contract-first, FE/BE drift yo'q |
| DB access | `pgx/v5` + `sqlc` | type-safe SQL, ORM'siz |
| Migratsiya | `goose` (embed) | binary ichida |
| Auth | `argon2id`, opaque session (cookie) + PAT; OIDC `coreos/go-oidc` | xavfsiz default |
| AuthZ | RBAC jadvallari + service qatlamida `authz.Can(ctx, action, resource)` | markaziy tekshiruv |
| Background jobs | `riverqueue/river` (Postgres-based) | Redis'siz ishlaydi |
| Realtime | WebSocket (`coder/websocket`) + Postgres `LISTEN/NOTIFY` / Redis pubsub | horizontal scale |
| Search | v0.5: KQL → SQL builder; full-text `tsvector` + `pg_trgm` | qo'shimcha infra'siz |
| Observability | `slog` (JSON), OpenTelemetry traces, Prometheus `/metrics` | |
| Config | env + `kyber.yaml` (`koanf`) | 12-factor |
| Test | `testing` + `testcontainers-go` (Postgres), golden files | haqiqiy DB bilan |

**Repo tuzilmasi:**
```
kyber/
├── api/openapi.yaml               # yagona kontrakt
├── cmd/kyber/main.go              # server, migrate, admin CLI (cobra)
├── internal/
│   ├── platform/                  # config, db, httpx, log, otel, authz, outbox
│   ├── identity/                  # users, sessions, tokens, oidc
│   ├── workspace/
│   ├── project/
│   ├── issue/                     # issue, comment, link, attachment, history
│   ├── workflow/
│   ├── agile/                     # board, sprint, backlog rank
│   ├── search/                    # kql lexer/parser/sql
│   ├── notify/
│   └── realtime/
├── migrations/                    # goose SQL
├── queries/                       # sqlc SQL
├── web/                           # React app (embed qilinadi)
├── deploy/                        # docker, compose, helm
└── docs/                          # PLAN, ADR, user docs
```

### 3.3 Ma'lumotlar modeli (asosiy jadvallar)
`users`, `sessions`, `api_tokens`, `workspaces`, `workspace_members`, `projects`, `project_members`,
`roles`, `issue_types`, `workflows`, `statuses`, `transitions`, `issues`
(`id uuid`, `project_id`, `number int` → `KEY-number`, `type_id`, `status_id`, `parent_id`,
`assignee_id`, `reporter_id`, `priority`, `rank text` (LexoRank), `sprint_id`, `custom jsonb`,
`search tsvector`, `version int` — optimistic locking), `comments`, `issue_links`, `labels`,
`issue_labels`, `attachments`, `sprints`, `boards`, `board_columns`, `watchers`,
`notifications`, `activity_log`, `outbox`, `webhooks`.

Muhim qarorlar:
- Issue raqami per-project ketma-ketlik: `UPDATE projects SET issue_seq = issue_seq + 1 RETURNING` (tranzaksiyada).
- Har bir yozish → `activity_log` + `outbox` bitta tranzaksiyada (history, realtime, webhook uchun).
- Tenant izolyatsiyasi: barcha query'larda `workspace_id`; v0.5 da Postgres RLS ko'rib chiqiladi (ADR).

### 3.4 Frontend (React)
| Soha | Tanlov |
|---|---|
| Build | Vite + React 19 + TypeScript (strict) |
| Routing | TanStack Router |
| Server state | TanStack Query + `openapi-fetch` (generatsiya qilingan tiplar) |
| UI | Tailwind CSS + Radix UI / shadcn/ui, dark mode |
| Drag & drop | `@dnd-kit` |
| Editor | TipTap (Markdown, `@mention`) |
| Forms | React Hook Form + Zod |
| Charts | Recharts (burndown, CFD) |
| i18n | `i18next` |
| Test | Vitest + Testing Library, Playwright (e2e), MSW |

Asosiy sahifalar: Login/Signup → Workspace switcher → Projects list → Board → Backlog →
Issue detail (modal + to'liq sahifa) → Search (KQL) → Project settings (workflow, members) →
Profile/Notifications. Klaviatura yorliqlari (`c` — yangi issue, `/` — qidiruv) birinchi kundan.

---

## 4. Yo'l xaritasi (milestone'lar)

Har bir milestone `awesome-agents` zanjiri bo'yicha o'tadi: `/design` → implement → `/review` → `/ship`.

### M0 — Poydevor (1-hafta)
- [ ] ADR-0001…0005 (§6) — *architecture-team*
- [ ] Monorepo skeleti, `Makefile`, `golangci-lint`, `pre-commit` — *platform-team*
- [ ] CI (GitHub Actions): lint, test, build, `govulncheck`, `npm audit`, CodeQL — *platform / security*
- [ ] `docker-compose.yml` (postgres, minio, mailpit), dev hot-reload (`air` + Vite proxy)
- [ ] OpenAPI skeleti + codegen pipeline; `/healthz`, `/readyz`, `/metrics`
- [ ] README, CONTRIBUTING, CODE_OF_CONDUCT, SECURITY.md, issue/PR shablonlari

### M1 — v0.1 MVP (2–5 hafta)
- [ ] F1 Auth (signup, login, logout, sessiya, PAT) + threat model — *backend + security*
- [ ] F2/F3 Workspace, Project, a'zolar, RBAC (F14 basic)
- [ ] F4 Issue CRUD, kalit generatsiyasi, sub-task, labels
- [ ] F5 Default workflow (To Do → In Progress → Done) — data-driven, hardcode emas
- [ ] F6 Kanban board, drag & drop, optimistic update + `version` conflict handling
- [ ] F8 Comments, F9 Activity log, F12 in-app notifications
- [ ] F11 Oddiy filterlar (assignee, status, type, label, text)
- [ ] Frontend: shell, auth, board, issue detail, project settings
- [ ] e2e: "signup → project → issue → board'da ko'chirish → comment" — *quality-team*
- [ ] **Release v0.1.0**: Docker image (distroless, multi-arch), GoReleaser — *release-team*

### M2 — v0.5 Beta (6–11 hafta)
- [ ] F7 Backlog, Sprint, Scrum board, LexoRank
- [ ] F5 Workflow editor (status/transition CRUD, vizual)
- [ ] F4 Custom fields (text, number, select, date, user)
- [ ] F10 Link turlari, Epic → children
- [ ] F11 **KQL**: lexer → parser (AST) → parametrlangan SQL; autocomplete endpoint
- [ ] F15 Realtime board (WebSocket), F12 email + watchers, F13 attachments
- [ ] F17 Webhooks (HMAC imzo, retry), F1 OIDC
- [ ] F16 Burndown; F19 uz/ru
- [ ] Load test (k6): 10k issue, 50 RPS → p95 < 100 ms

### M3 — v1.0 (12–16 hafta)
- [ ] Saved filters, swimlanes, quick filters, CFD/velocity
- [ ] Permission schemes, issue-level security; audit log UI
- [ ] F18 Jira import (CSV/JSON); F17 GitHub integratsiya (commit `KYB-12` → link)
- [ ] Helm chart, backup/restore hujjati, upgrade yo'riqnomasi
- [ ] Pentest + `/audit`, `/ship` gate; docs sayt (Docusaurus / VitePress)
- [ ] **Release v1.0.0** + launch (HN, Reddit r/selfhosted, Product Hunt)

---

## 5. Agentlar bilan ishlash tartibi (awesome-agents)

O'rnatish (`.claude/settings.json` da allaqachon yoqilgan):
```
/plugin marketplace add bakhod1r/awesome-agents
/plugin install agent-workflow@awesome-agents
```

| Bosqich | Buyruq / Agent | Natija (artefakt) |
|---|---|---|
| Discovery | `product-team` (product-manager, business-analyst) | User story + acceptance criteria (GitHub issue) |
| Dizayn | `/design <feature>` → backend/frontend/database-architect | `docs/adr/NNNN-*.md`, OpenAPI diff, DB migratsiya rejasi |
| Xavfsizlik | `/threat-model <feature>` (auth, permissions, webhooks, attachments) | STRIDE jadval + mitigations |
| UI | `design-team` → `frontend-team` | Wireframe → komponentlar |
| Implementatsiya | `backend-team`, `frontend-team` | Kod + unit/integration testlar |
| Tekshiruv | `/review`, `quality-team` | Review hisobot, e2e, contract testlar |
| Reliz | `/ship`, `release-team` | Readiness gate, CHANGELOG, tag |

Katta feature'lar uchun: `/flow "Sprint va backlog moduli"` — har bosqichdan keyin tasdiq.

---

## 6. Dastlabki ADR'lar (M0 da yoziladi)

| ADR | Mavzu | Taklif |
|---|---|---|
| 0001 | Arxitektura uslubi | Modular monolith, single binary, embedded SPA |
| 0002 | DB access | `pgx` + `sqlc` + `goose`, ORM yo'q |
| 0003 | API | Contract-first OpenAPI 3.1, REST + WebSocket |
| 0004 | Multi-tenancy | Shared schema + `workspace_id`; RLS keyinroq |
| 0005 | Auth | Opaque session cookie (web) + PAT (API), argon2id |
| 0006 | Ranking | LexoRank (string), qayta balanslash job |
| 0007 | Litsenziya | **AGPL-3.0** (SaaS forklardan himoya) yoki MIT — *qaror kerak* |

---

## 7. Sifat, xavfsizlik, operatsiya

- **Testlar:** domain unit → service (testcontainers) → HTTP contract (OpenAPI validator) → Playwright e2e.
- **Xavfsizlik:** OWASP ASVS L2; CSRF token, `SameSite=Lax`, rate limit (login), CSP, input
  validation (OpenAPI), parametrlangan SQL, attachment MIME tekshiruvi, secret scanning, SBOM (syft), cosign imzo.
- **Observability:** trace_id har log'da; RED metrikalar; Grafana dashboard JSON `deploy/` da.
- **Migratsiyalar:** faqat oldinga, backward-compatible (expand → contract).
- **Versiyalash:** SemVer, Conventional Commits, `release-please` / GoReleaser.

---

## 8. Open-source ko'tarish (community)

1. **README** — GIF demo, 1 buyruqli start, Jira bilan solishtirish jadvali.
2. **Live demo** — `demo.kyber.dev` (har kecha reset).
3. **Good first issues** — har milestone'da 10+ ta, `help wanted` label.
4. **Docs** — Getting started, self-hosting, API reference (OpenAPI'dan avtomatik), KQL qo'llanma.
5. **Kanallar** — GitHub Discussions, Telegram (uz/ru community), Discord.
6. **Launch** — v0.1 da r/selfhosted, v1.0 da Hacker News "Show HN", Product Hunt; awesome-selfhosted ro'yxatiga PR.
7. **Governance** — `MAINTAINERS.md`, RFC jarayoni katta o'zgarishlar uchun (`docs/rfcs/`).

---

## 9. Xavflar

| Xavf | Ehtimol | Ta'sir | Choralar |
|---|---|---|---|
| Scope creep (Jira juda katta) | Yuqori | Yuqori | §1.3 qat'iy; har feature uchun issue + milestone |
| KQL murakkabligi | O'rta | O'rta | Grammatikani kichik boshlash, fuzz test |
| Board'da konkurent tahrir | O'rta | O'rta | Optimistic locking (`version`), realtime sync |
| Bitta maintainer yuklamasi | Yuqori | Yuqori | Agent zanjiri, aniq CONTRIBUTING, avtomatlashtirilgan CI |

---

## 10. Keyingi qadamlar (darhol)

1. ADR-0007 (litsenziya) bo'yicha qaror.
2. M0: repo skeleti + CI + docker-compose.
3. `/design "Kyber core domain model"` → ADR-0001…0005.
4. GitHub'da M1 issue'larini milestone bilan ochish.
