# QA Report — Sprint 07

**Teams:** Quality + Design + Security · **Verdict:** ✅ Ready for **v0.7.0**

## Acceptance criteria coverage
| Story | Evidence | Result |
|---|---|---|
| S25 Estimates & sprint dates | `ParsePoints` table test (0.1 steps, 0–999), `SetEstimate` events, `Start(at, endsAt)` invariant; `TestS25EstimatesAndSprintDates` ×3 (2.5 pts, −1/1000/1.25 → 422, null clears, past end → 422, default 14 days) | PASS |
| S26 Activity log | ingestion test (all four events, float → tenths, redelivery, malformed skipped); repo contract ×2; **migration backfill from real outbox rows** | PASS |
| S27 Reports API | report maths on hand-computed fixtures (created vs resolved incl. re-open, cycle time incl. re-open + window, burndown with add / re-estimate / removal / foreign issue, velocity); `TestS27Reports` ×3: 10 committed → 3 done → +8 added → **15 remaining**, velocity 10/3, outsiders 404, `days` validation | PASS |
| S28 Reports UI | 9 Vitest tests (KPI tiles, 7 labelled figures each with a data table, date-range refetch, empty states, story points in panel/backlog, sprint end date); chart primitives tests (nice axis, bar scaling, legend, crosshair tooltip, malformed points); e2e seeding a sprint and reading KPIs, burndown table and velocity in the browser | PASS |

## Data-visualization checks (dataviz method)
| Check | Result |
|---|---|
| Palette validator, light (surface `#ffffff`) | slots 1–2 all PASS (CVD ΔE 24.7, normal 33.6, contrast ≥ 3:1) |
| Palette validator, dark (surface `#111b2a`) | selected dark steps all PASS (CVD ΔE 26.8, normal 31.8) |
| Form | KPI stat tiles; single-series bar lists (no legend); 2-series lines with legend + end labels; ideal as a muted solid series; grouped columns; **no dual axis** |
| Marks | bars ≤ 24 px with 4 px rounded data-end, 2 px lines, ≥ 8 px end dots with 2 px surface ring, hairline solid grid |
| Interaction / a11y | crosshair tooltip listing every series; every chart is a `figure` with a heading and a data table; text never in series colour |
| Rendered & inspected | light + dark screenshots; fixed oversized axis text (viewBox sizing) before shipping |

## Found & fixed this sprint
| ID | Severity | Finding | Fix |
|---|---|---|---|
| QA-07-1 | Medium | A malformed date in report data crashed the whole chart (`Invalid time value`) | LineChart drops invalid points; regression test |
| QA-07-2 | Low | Line/column chart text rendered oversized because the SVG scaled up to the card width | realistic viewBox widths (800) |

## Notes
- History is complete from v0.2 on PostgreSQL thanks to the outbox backfill; in in-memory dev mode it starts at server start.
- A sprint cannot be started with an end date in the past (by design); seeding historic demo data therefore needs backdating.
- Velocity counts story points; unestimated issues count as 0 points (issue counts are in the data table).
