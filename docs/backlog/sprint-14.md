# Sprint 14 — A better whiteboard

Source: product request (2026-10-14, "whiteboardni takomillashtir"). The S59/S63/S86 whiteboard draws shapes, arrows,
pen, text and sticky notes together, but on a fixed 1600×900 page, one element at a time, without resizing or
styles. This sprint brings it closer to Miro/Excalidraw. Planned first, then built DDD + TDD (pure model functions in
`web/src/contexts/whiteboard/domain`, contract and acceptance tests where the server changes, Playwright e2e).

- ✅ **S89 Infinite canvas: pan and zoom** — the board is no longer a fixed page (coordinates already allow ±10⁶).
  AC: wheel/pinch zooms around the pointer (10 %–400 %); drag with the Hand tool, the middle button or Space+drag
  pans; "Fit" shows every element; zoom buttons and a percentage; new shapes land where you look; export covers
  every element, not the viewport.
  **Done:** `View` model (`zoomAt`, `panBy`, `fit`, `bounds`) in the domain; wheel/pinch zoom at the pointer,
  ✋ Hand tool, middle button and Space+drag pan, Ctrl +/−/0 and Shift+1; zoom bar with −, %, +, Fit; dot grid;
  export of the whole drawing.
- ✅ **S90 Resize, multi-select, copy** — AC: eight handles resize a box (rect, ellipse, text, sticky), line/arrow
  ends can be dragged; Shift+click and a marquee on empty space select several; moving, deleting, recolouring and
  undo work on the whole selection (one undo step); Ctrl+D duplicates, Ctrl+C/Ctrl+V copy and paste (also between
  boards); arrow keys nudge (Shift = 10 px).
  **Done:** `resize`, `moveEnd`, `moveBy`, `within`, `duplicate`, clipboard and `inverseAll` in the domain;
  `changeAll` makes a batch one undo step; handles, marquee, Shift+click, Ctrl+A/D/C/X/V, Esc, arrows; copies draw
  on top. e2e: resize by handle, marquee, duplicate, nudge, delete and undo.
- ✅ **S91 Styles** — stroke width (thin/medium/bold), filled or outlined shapes, dashed lines, and a larger palette
  (12 colours, sticky colours). AC: stored with the element (server validates), drawn the same in export.
  **Done:** element `stroke` (0–3), `fill`, `dashed` (domain validation, migration 0077, API 0.66.0); Style bar
  (Thin/Medium/Bold, Fill, Dashed) restyles the selection and new shapes; 12 colours; six sticky colours.
- ✅ **S92 Templates** — start a board from Retrospective (went well / to improve / actions), Kanban
  (to do / doing / done), SWOT or a blank page; a template is a batch of ordinary elements anyone can edit.
  **Done:** `template()` in the domain; "Template…" inserts it in the middle of the view, selected, one undo step.
- ✅ **S93 Eraser and order** — an eraser tool (drag over strokes to delete them, one undo step); bring to front /
  send to back (stored order, not the last change).
  **Done:** element `z` (migration 0078, API 0.67.0) orders shapes, ties by last change; ⬆/⬇ and Ctrl+]/[; the ⌫
  Eraser fades what it touches and deletes it on release, one undo step. Also fixed: a fast drag on a busy
  machine could save a stale shape (pointerup read state React had not rendered yet).
- **Fix (S88 follow-up):** the web stream now reloads only after the broker granted its subscriptions on each
  (re)connect; an event in that gap (e.g. "huddle started" right after opening a channel) was lost.

Order: S89 → S90 (they change how every interaction maps to coordinates), then S91, S92, S93.
