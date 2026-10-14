import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, type PointerEvent, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { type MessageKey, msg, useT } from "../../../shared/i18n/i18n";
import { issueUseCases } from "../../issue/application";
import { projectUseCases } from "../../project/application";
import {
  bounds, box, COLORS, commentCounts, duplicate, type Element, erased, fit, fitImage, imageURL, fromClipboard, type Handle, HANDLES, hit, HOME, type Kind, linkedIssue, moveBy, moveEnd, newId,
  look, onScreen, onWhiteboardComments, ordered, panBy, resize, restyle, STICKY_COLORS, type Template, template, TEMPLATES, toBack, toClipboard, toFront, within,
  type Point, STICKY, stickyToIssue, useWhiteboard, type View, viewBox, whiteboardUseCases, zoomAt,
} from "../application";

const W = 1600;
const H = 900;
const PAGE = { w: W, h: H };
type Tool = "select" | "hand" | "eraser" | Kind;
const TOOLS: { tool: Tool; label: MessageKey; icon: string }[] = [
  { tool: "select", label: msg("Select"), icon: "↖" },
  { tool: "hand", label: msg("Hand"), icon: "✋" },
  { tool: "rect", label: msg("Rectangle"), icon: "▭" },
  { tool: "ellipse", label: msg("Ellipse"), icon: "◯" },
  { tool: "arrow", label: msg("Arrow"), icon: "→" },
  { tool: "line", label: msg("Line"), icon: "／" },
  { tool: "freehand", label: msg("Pen"), icon: "✎" },
  { tool: "text", label: msg("Text"), icon: "T" },
  { tool: "sticky", label: msg("Sticky note"), icon: "🗒" },
  { tool: "eraser", label: msg("Eraser"), icon: "⌫" },
];
/** Template names shown in the picker (the domain keeps its own English labels). */
const TEMPLATE_LABELS: Record<Template, MessageKey> = { retro: msg("Retrospective"), kanban: msg("Kanban"), swot: msg("SWOT") };
const STROKE_LABELS: Record<1 | 2 | 3, MessageKey> = { 1: msg("Thin"), 2: msg("Medium"), 3: msg("Bold") };
const CURSOR_COLORS = ["#e11d48", "#7c3aed", "#0891b2", "#ca8a04", "#059669"];

/**
 * The whiteboards of a conversation (S59): pick or create one, then draw together — shapes, arrows, pen,
 * text and sticky notes, with everyone's changes and cursors live. Slotted into the chat by the app shell.
 */
export function Whiteboards({ conversationId, me, names }: { conversationId: string; me: string; names: Map<string, string> }) {
  const t = useT();
  const qc = useQueryClient();
  const [open, setOpen] = useState<string | null>(null);
  const [shown, setShown] = useState(false);
  const [title, setTitle] = useState("");
  const list = useQuery({ queryKey: ["whiteboards", conversationId], queryFn: () => whiteboardUseCases.list(conversationId), enabled: shown });
  const create = useMutation({
    mutationFn: () => whiteboardUseCases.create(conversationId, title),
    onSuccess: (b) => {
      setTitle("");
      setOpen(b.id);
      return qc.invalidateQueries({ queryKey: ["whiteboards", conversationId] });
    },
  });
  if (!shown) return <button className="ghost" onClick={() => setShown(true)}>{t("Whiteboards")}</button>;
  if (open) return <Board id={open} me={me} names={names} onClose={() => setOpen(null)} />;
  const submit = (e: FormEvent) => {
    e.preventDefault();
    create.mutate();
  };
  return (
    <section className="wb-list panel" aria-label={t("Whiteboards")}>
      <header className="row">
        <h2>{t("Whiteboards")}</h2>
        <button className="ghost" aria-label={t("Close whiteboards")} onClick={() => setShown(false)}>✕</button>
      </header>
      <ul>
        {list.data?.length === 0 && <li className="muted">{t("No whiteboards yet.")}</li>}
        {list.data?.map((b) => (
          <li key={b.id}><button className="link" onClick={() => setOpen(b.id)}>{b.title}</button> <span className="muted">{t("by {name}", { name: names.get(b.created_by) ?? t("someone") })}</span></li>
        ))}
      </ul>
      <form onSubmit={submit} className="row">
        <input aria-label={t("Whiteboard title")} placeholder={t("Architecture")} value={title} onChange={(e) => setTitle(e.target.value)} required />
        <button>{t("New whiteboard")}</button>
      </form>
      {create.error && <p role="alert" className="error">{create.error.message}</p>}
    </section>
  );
}

function Shape({ e, selected, board }: { e: Element; selected: boolean; board: string }) {
  const stroke = e.color || "#1f2937";
  const l = look(e);
  const common = { "data-id": e.id, stroke, strokeWidth: l.strokeWidth + (selected ? 1 : 0), fill: l.fill, strokeDasharray: l.dash } as const;
  const pts = e.points.map((p) => `${e.x + p.x},${e.y + p.y}`).join(" ");
  switch (e.kind) {
    case "rect":
      return <g><rect {...common} x={e.x} y={e.y} width={e.w} height={e.h} rx={6} /><Label e={e} /></g>;
    case "ellipse":
      return <g><ellipse {...common} cx={e.x + e.w / 2} cy={e.y + e.h / 2} rx={e.w / 2} ry={e.h / 2} /><Label e={e} /></g>;
    case "line":
      return <polyline {...common} fill="none" points={pts} />;
    case "arrow":
      return <polyline {...common} fill="none" points={pts} markerEnd="url(#wb-arrow)" />;
    case "freehand":
      return <polyline {...common} fill="none" points={pts} strokeLinecap="round" strokeLinejoin="round" />;
    case "sticky":
      return (
        <g>
          <rect data-id={e.id} x={e.x} y={e.y} width={e.w} height={e.h} fill={STICKY_COLORS.includes(e.color) ? e.color : STICKY}
            stroke={selected ? "#1f2937" : "#00000022"} strokeWidth={selected ? 2 : 1} />
          <Label e={e} />
        </g>
      );
    case "image":
      return (
        <g>
          <image data-id={e.id} href={imageURL(board, e)} x={e.x} y={e.y} width={e.w} height={e.h} preserveAspectRatio="none" />
          {selected && <rect x={e.x} y={e.y} width={e.w} height={e.h} fill="none" stroke="#2563eb" strokeWidth={2} pointerEvents="none" />}
        </g>
      );
    case "text":
      return <g><rect data-id={e.id} x={e.x} y={e.y} width={e.w} height={e.h} fill="transparent" stroke={selected ? "#94a3b8" : "none"} strokeDasharray="4" /><Label e={e} /></g>;
  }
}

function Label({ e }: { e: Element }) {
  if (!e.text) return null;
  const lines = e.text.split("\n");
  return (
    <text x={e.x + 10} y={e.y + 24} fill={e.kind === "text" ? e.color || "#1f2937" : "#1f2937"} fontSize={18} pointerEvents="none">
      {lines.map((l, i) => <tspan key={i} x={e.x + 10} dy={i === 0 ? 0 : 22}>{l}</tspan>)}
    </text>
  );
}

/** S90: handles on the one selected element — eight to resize a box, one per end of a line or arrow. */
function Handles({ e, zoom }: { e: Element; zoom: number }) {
  const t = useT();
  const r = 6 / zoom;
  if (e.kind === "line" || e.kind === "arrow") {
    return (
      <g>
        {[0, e.points.length - 1].map((i) => (
          <circle key={i} data-handle={`end-${i}`} aria-label={i === 0 ? t("Line start") : t("Line end")} className="wb-handle" cx={e.x + e.points[i]!.x} cy={e.y + e.points[i]!.y} r={r} />
        ))}
      </g>
    );
  }
  if (e.kind === "freehand") return null;
  const at: Record<Handle, [number, number]> = {
    nw: [0, 0], n: [0.5, 0], ne: [1, 0], e: [1, 0.5], se: [1, 1], s: [0.5, 1], sw: [0, 1], w: [0, 0.5],
  };
  return (
    <g>
      {HANDLES.map((h) => (
        <rect key={h} data-handle={h} aria-label={t("Resize {handle}", { handle: h })} className={`wb-handle wb-handle-${h}`}
          x={e.x + at[h][0] * e.w - r} y={e.y + at[h][1] * e.h - r} width={2 * r} height={2 * r} />
      ))}
    </g>
  );
}

/** Types an element's text in place over the canvas: Enter (Shift+Enter for a new line) or leaving saves, Escape cancels. */
function TextEditor({ e, view, onDone }: { e: Element; view: View; onDone: (text: string | null) => void }) {
  const t = useT();
  const done = useRef(false);
  const finish = (text: string | null) => {
    if (done.current) return;
    done.current = true;
    onDone(text);
  };
  const pct = (v: number, of: number) => `${(v / of) * 100}%`;
  return (
    <textarea className={`wb-editor ${e.kind === "sticky" ? "sticky" : ""}`} aria-label={e.kind === "sticky" ? t("Sticky note text") : t("Text")} autoFocus
      defaultValue={e.text} placeholder={e.kind === "sticky" ? t("Sticky note") : t("Type…")}
      style={{ left: `${onScreen(view, PAGE, e).left}%`, top: `${onScreen(view, PAGE, e).top}%`, width: pct(Math.max(e.w, 120) * view.zoom, W),
        minHeight: pct(Math.max(e.h, 34) * view.zoom, H), fontSize: `${view.zoom}em`, color: e.kind === "sticky" ? undefined : e.color }}
      onKeyDown={(k) => {
        if (k.key === "Escape") finish(null);
        else if (k.key === "Enter" && !k.shiftKey) {
          k.preventDefault();
          finish(k.currentTarget.value);
        }
      }}
      onBlur={(b) => finish(b.currentTarget.value)} />
  );
}

/** One whiteboard to draw on (also what a huddle shows when it is presented). */
export function WhiteboardBoard(props: { id: string; me: string; names: Map<string, string>; onClose?: () => void }) {
  return <Board {...props} />;
}

/** S86: the comments on the selected element; anyone in the conversation adds one, its author removes it. */
function ElementComments({ board, element, me, names }: { board: string; element: string; me: string; names: Map<string, string> }) {
  const t = useT();
  const qc = useQueryClient();
  const key = ["whiteboard", "comments", board];
  const list = useQuery({ queryKey: key, queryFn: () => whiteboardUseCases.comments(board) });
  const [body, setBody] = useState("");
  const done = () => qc.invalidateQueries({ queryKey: key });
  const add = useMutation({ mutationFn: () => whiteboardUseCases.comment(board, element, body), onSuccess: () => { setBody(""); void done(); } });
  const remove = useMutation({ mutationFn: (id: string) => whiteboardUseCases.removeComment(id), onSuccess: done });
  const mine = (list.data ?? []).filter((c) => c.element_id === element);
  return (
    <aside className="wb-comments panel" aria-label={t("Element comments")}>
      <h3>💬 {t("Comments")}</h3>
      {mine.length === 0 && <p className="muted">{t("No comments yet.")}</p>}
      <ul>
        {mine.map((c) => (
          <li key={c.id}>
            <b>{c.author_id === me ? t("You") : (names.get(c.author_id) ?? t("Someone"))}</b> {c.body}{" "}
            {c.author_id === me && <button className="ghost" onClick={() => remove.mutate(c.id)}>{t("Remove")}</button>}
          </li>
        ))}
      </ul>
      <form className="row" onSubmit={(e) => { e.preventDefault(); add.mutate(); }}>
        <input aria-label={t("Comment")} value={body} onChange={(e) => setBody(e.target.value)} placeholder={t("Comment… (@email to mention)")} maxLength={2000} />
        <button disabled={!body.trim()}>{t("Comment")}</button>
      </form>
    </aside>
  );
}

function Board({ id, me, names, onClose }: { id: string; me: string; names: Map<string, string>; onClose?: () => void }) {
  const t = useT();
  const wb = useWhiteboard(id, me);
  const qc = useQueryClient();
  const comments = useQuery({ queryKey: ["whiteboard", "comments", id], queryFn: () => whiteboardUseCases.comments(id) });
  useEffect(() => onWhiteboardComments((board) => {
    if (board === id) void qc.invalidateQueries({ queryKey: ["whiteboard", "comments", id] });
  }), [id, qc]);
  const counts = commentCounts(comments.data ?? []);
  const svg = useRef<SVGSVGElement>(null);
  const [tool, setTool] = useState<Tool>("select");
  const [color, setColor] = useState(COLORS[0]!);
  /** S91: the style new shapes get; changing it restyles the selection too. */
  const [style, setStyle] = useState<Pick<Element, "stroke" | "fill" | "dashed">>({ stroke: 0, fill: false, dashed: false });
  /** S90: the selected elements (Shift+click or a marquee selects several). */
  const [selected, setSelectedIds] = useState<string[]>([]);
  const setSelected = (id: string | null) => setSelectedIds(id ? [id] : []);
  /** S89: the part of the infinite board on screen; pan holds a drag of the board in progress. */
  const [view, setView] = useState<View>(HOME);
  const pan = useRef<{ x: number; y: number; from: View } | null>(null);
  const space = useRef(false);
  const [draft, setDraftState] = useState<Element | null>(null); // the shape being drawn (mirrored like moving, below)
  const draftRef = useRef<Element | null>(null);
  const setDraft = (next: Element | null | ((cur: Element | null) => Element | null)) => {
    draftRef.current = typeof next === "function" ? next(draftRef.current) : next;
    setDraftState(draftRef.current);
  };
  /** What a drag in the select tool does: move the selection, resize a box, move a line end, or draw a marquee. */
  const drag = useRef<
    | { mode: "draw"; start: Point; origin: Element }
    | { mode: "move"; start: Point; origins: Element[] }
    | { mode: "resize"; start: Point; origin: Element; handle: Handle }
    | { mode: "end"; origin: Element; index: number }
    | { mode: "marquee"; start: Point; add: string[] }
    | { mode: "erase"; path: Point[] }
    | null
  >(null);
  /**
   * Elements being moved or reshaped, the marquee, and what the eraser passed over. Each is mirrored in a ref: pointerup
   * must see the last pointermove even when React has not rendered it yet (a fast drag on a busy machine).
   */
  const [moving, setMovingState] = useState<Element[] | null>(null);
  const movingRef = useRef<Element[] | null>(null);
  const setMoving = (m: Element[] | null) => {
    movingRef.current = m;
    setMovingState(m);
  };
  const [erasing, setErasingState] = useState<string[]>([]); // S93
  const erasingRef = useRef<string[]>([]);
  const setErasing = (ids: string[]) => {
    erasingRef.current = ids;
    setErasingState(ids);
  };
  const [marquee, setMarqueeState] = useState<{ x: number; y: number; w: number; h: number } | null>(null);
  const marqueeRef = useRef<{ x: number; y: number; w: number; h: number } | null>(null);
  const setMarquee = (m: { x: number; y: number; w: number; h: number } | null) => {
    marqueeRef.current = m;
    setMarqueeState(m);
  };
  /** The element whose text is being typed in place (S63); fresh when it is not on the board yet. */
  const [editing, setEditing] = useState<{ element: Element; fresh: boolean } | null>(null);
  const live = (id: string) => moving?.find((m) => m.id === id) ?? wb.elements[id];
  const selection = selected.map(live).filter((e): e is Element => !!e);
  const sel = selection.length === 1 ? selection[0] : undefined;

  const at = (e: PointerEvent): Point => {
    const m = svg.current?.getScreenCTM();
    if (!m) return { x: 0, y: 0 };
    const p = new DOMPoint(e.clientX, e.clientY).matrixTransform(m.inverse());
    return { x: Math.round(p.x), y: Math.round(p.y) };
  };

  /** page converts a distance in screen pixels to page units (the SVG is scaled to fit its box). */
  const page = (dx: number, dy: number) => {
    const r = svg.current?.getBoundingClientRect();
    return r && r.width > 0 ? { dx: (dx * W) / r.width, dy: (dy * H) / r.height } : { dx, dy };
  };
  const down = (e: PointerEvent<SVGSVGElement>) => {
    const p = at(e);
    svg.current?.setPointerCapture?.(e.pointerId);
    if (tool === "hand" || e.button === 1 || space.current) {
      e.preventDefault();
      pan.current = { x: e.clientX, y: e.clientY, from: view };
      return;
    }
    if (tool === "select") {
      const handle = (e.target as globalThis.Element).closest?.("[data-handle]")?.getAttribute("data-handle");
      if (handle && sel) {
        drag.current = handle.startsWith("end-")
          ? { mode: "end", origin: sel, index: Number(handle.slice(4)) }
          : { mode: "resize", start: p, origin: sel, handle: handle as Handle };
        return;
      }
      const target = [...ordered(wb.elements)].reverse().find((x) => hit(x, p));
      if (!target) {
        if (!e.shiftKey) setSelectedIds([]);
        drag.current = { mode: "marquee", start: p, add: e.shiftKey ? selected : [] };
        return;
      }
      if (e.shiftKey) {
        setSelectedIds(selected.includes(target.id) ? selected.filter((x) => x !== target.id) : [...selected, target.id]);
        return;
      }
      const ids = selected.includes(target.id) ? selected : [target.id];
      setSelectedIds(ids);
      drag.current = { mode: "move", start: p, origins: ids.map((x) => wb.elements[x]).filter((x): x is Element => !!x) };
      return;
    }
    if (tool === "eraser") {
      drag.current = { mode: "erase", path: [p] };
      setErasing(erased(Object.values(wb.elements), [p]).map((x) => x.id));
      return;
    }
    const fillable = tool === "rect" || tool === "ellipse";
    const base: Element = { id: newId(), kind: tool, x: p.x, y: p.y, w: 0, h: 0, points: [] as Point[], text: "", color, version: 0,
      stroke: style.stroke, dashed: style.dashed, ...(fillable ? { fill: style.fill } : {}) };
    if (tool === "text" || tool === "sticky") {
      e.preventDefault(); // keep the focus for the editor
      const el: Element = { ...base, w: tool === "sticky" ? 180 : 240, h: tool === "sticky" ? 120 : 34, color: tool === "sticky" ? STICKY : color };
      setEditing({ element: el, fresh: true });
      setTool("select");
      return;
    }
    drag.current = { mode: "draw", start: p, origin: base };
    setDraft({ ...base, points: tool === "freehand" || tool === "line" || tool === "arrow" ? [{ x: 0, y: 0 }] : [] });
  };

  const move = (e: PointerEvent<SVGSVGElement>) => {
    if (pan.current) {
      const { dx, dy } = page(e.clientX - pan.current.x, e.clientY - pan.current.y);
      setView(panBy(pan.current.from, dx, dy));
      return;
    }
    const p = at(e);
    wb.moveCursor(p.x, p.y);
    const d = drag.current;
    if (!d) return;
    if (d.mode === "move") return setMoving(moveBy(d.origins, p.x - d.start.x, p.y - d.start.y));
    if (d.mode === "resize") return setMoving([resize(d.origin, d.handle, p.x - d.start.x, p.y - d.start.y)]);
    if (d.mode === "end") return setMoving([moveEnd(d.origin, d.index, p)]);
    if (d.mode === "marquee") return setMarquee(box(d.start.x, d.start.y, p.x, p.y));
    if (d.mode === "erase") {
      d.path.push(p);
      return setErasing([...new Set([...erasingRef.current, ...erased(Object.values(wb.elements), [p]).map((x) => x.id)])]);
    }
    const o = d.origin;
    if (tool === "freehand") {
      setDraft((cur) => (cur ? { ...cur, points: [...cur.points, { x: p.x - o.x, y: p.y - o.y }] } : cur));
    } else if (tool === "line" || tool === "arrow") {
      setDraft({ ...o, points: [{ x: 0, y: 0 }, { x: p.x - o.x, y: p.y - o.y }] });
    } else {
      setDraft({ ...o, ...box(d.start.x, d.start.y, p.x, p.y) });
    }
  };

  const up = () => {
    pan.current = null;
    const d = drag.current;
    drag.current = null;
    if (d?.mode === "marquee") {
      const area = marqueeRef.current;
      setMarquee(null);
      if (area && (area.w > 3 || area.h > 3)) setSelectedIds([...new Set([...d.add, ...within(Object.values(wb.elements), area).map((x) => x.id)])]);
      return;
    }
    if (d?.mode === "erase") {
      wb.changeAll(erasingRef.current.map((x) => wb.elements[x]).filter((x): x is Element => !!x).map((element) => ({ op: "delete", element })));
      setErasing([]);
      return;
    }
    if (d && d.mode !== "draw") {
      const changed = (movingRef.current ?? []).filter((m) => {
        const o = wb.elements[m.id];
        return o && (o.x !== m.x || o.y !== m.y || o.w !== m.w || o.h !== m.h || o.points !== m.points);
      });
      wb.changeAll(changed.map((element) => ({ op: "update", element })));
      setMoving(null);
      return;
    }
    const drawn = draftRef.current;
    if (d && drawn && (drawn.points.length > 1 || drawn.w > 3 || drawn.h > 3)) wb.change({ op: "add", element: drawn });
    setDraft(null);
  };

  const projects = useQuery({ queryKey: ["projects"], queryFn: projectUseCases.projects });
  const toIssue = useMutation({
    mutationFn: async ({ note, key }: { note: Element; key: string }) => {
      const { title, description } = stickyToIssue(note.text);
      const issue = await issueUseCases.createIssue(key, title, "task");
      if (description) await issueUseCases.editIssue(issue.key, { version: issue.version, description });
      return { note, issue };
    },
    onSuccess: ({ note, issue }) => wb.change({ op: "update", element: { ...note, text: `${note.text}\n→ ${issue.key}` } }),
  });
  const remove = () => {
    wb.changeAll(selection.map((element) => ({ op: "delete", element })));
    setSelectedIds([]);
  };
  /** Adds copies of elements (duplicate, paste) and selects them. */
  const addCopies = (els: Element[], offset: number) => {
    const copies = duplicate(els, offset);
    wb.changeAll(copies.map((element) => ({ op: "add", element })));
    setSelectedIds(copies.map((c) => c.id));
  };
  const clipboard = useRef("");
  const copy = () => {
    if (selection.length === 0) return;
    clipboard.current = toClipboard(selection);
    void navigator.clipboard?.writeText(clipboard.current).catch(() => undefined);
  };
  const paste = async () => {
    const text = await navigator.clipboard?.readText().catch(() => "") || clipboard.current;
    const els = fromClipboard(text);
    if (els.length > 0) addCopies(els, 24);
  };
  const nudge = (dx: number, dy: number) => wb.changeAll(moveBy(selection, dx, dy).map((element) => ({ op: "update", element })));
  /**
   * naturalSize reads a picture's size (0×0 when the browser cannot decode it). createImageBitmap decodes the file
   * itself; an <img> on a blob: URL would be blocked by the page's Content-Security-Policy (img-src).
   */
  const naturalSize = async (file: File): Promise<{ w: number; h: number }> => {
    if (typeof createImageBitmap === "function") {
      try {
        const bmp = await createImageBitmap(file);
        const size = { w: bmp.width, h: bmp.height };
        bmp.close();
        return size;
      } catch {
        return { w: 0, h: 0 };
      }
    }
    return new Promise<{ w: number; h: number }>((resolve) => {
      const url = URL.createObjectURL(file);
      const img = new Image();
      const done = (w: number, h: number) => {
        URL.revokeObjectURL(url);
        resolve({ w, h });
      };
      img.onload = () => done(img.naturalWidth, img.naturalHeight);
      img.onerror = () => done(0, 0);
      img.src = url;
    });
  };
  const editable = (e?: Element) => !!e && (e.kind === "text" || e.kind === "sticky" || e.kind === "rect" || e.kind === "ellipse");
  const editText = () => {
    if (sel && editable(sel)) setEditing({ element: sel, fresh: false });
  };
  /** Saves the typed text: a new element is added (unless empty), an existing one updated. */
  const finishEditing = (text: string | null) => {
    const ed = editing;
    setEditing(null);
    if (!ed || text === null) return;
    const el = ed.element;
    if (ed.fresh) {
      if (!text.trim()) return;
      const w = el.kind === "text" ? Math.max(60, Math.max(...text.split("\n").map((l) => l.length)) * 10) : el.w;
      const h = el.kind === "text" ? 34 * text.split("\n").length : el.h;
      wb.change({ op: "add", element: { ...el, text, w, h } });
      setSelected(el.id);
    } else if (text !== el.text) {
      wb.change({ op: "update", element: { ...(wb.elements[el.id] ?? el), text } });
    }
  };
  /** S92: lays a template out in the middle of what is on screen, selected so it can be moved at once. */
  /** S96: uploads a picture and places it, fitted, in the middle of the view, selected. */
  const [imageError, setImageError] = useState("");
  const fileInput = useRef<HTMLInputElement>(null);
  const addImage = async (file: File) => {
    if (!file.type.startsWith("image/")) return;
    setImageError("");
    try {
      const [up, natural] = await Promise.all([whiteboardUseCases.uploadImage(id, file), naturalSize(file)]);
      const size = fitImage(natural.w, natural.h);
      const c = { x: view.x + W / view.zoom / 2, y: view.y + H / view.zoom / 2 };
      const el: Element = { id: newId(), kind: "image", x: Math.round(c.x - size.w / 2), y: Math.round(c.y - size.h / 2), ...size, points: [], text: up.id, color: "", version: 0 };
      wb.change({ op: "add", element: el });
      setSelectedIds([el.id]);
    } catch (e) {
      setImageError(e instanceof Error ? e.message : t("Could not add the image"));
    }
  };
  useEffect(() => {
    const paste = (e: ClipboardEvent) => {
      if ((e.target as HTMLElement).closest?.("input, textarea")) return;
      const file = [...(e.clipboardData?.files ?? [])].find((f) => f.type.startsWith("image/"));
      if (file) {
        e.preventDefault();
        void addImage(file);
      }
    };
    window.addEventListener("paste", paste);
    return () => window.removeEventListener("paste", paste);
  });
  const insertTemplate = (kind: Template) => {
    const els = template(kind, { x: view.x + W / view.zoom / 2, y: view.y + H / view.zoom / 2 });
    wb.changeAll(els.map((element) => ({ op: "add", element })));
    setSelectedIds(els.map((e) => e.id));
  };
  const applyStyle = (patch: Pick<Element, "stroke" | "fill" | "dashed">) => {
    setStyle((s) => ({ ...s, ...patch }));
    wb.changeAll(restyle(selection, patch).map((element) => ({ op: "update", element })));
  };
  const recolorNotes = (c: string) =>
    wb.changeAll(selection.filter((x) => x.kind === "sticky").map((x) => ({ op: "update", element: { ...x, color: c } })));
  const recolor = (c: string) => {
    setColor(c);
    wb.changeAll(selection.filter((x) => x.kind !== "sticky").map((x) => ({ op: "update", element: { ...x, color: c } })));
  };

  /** zoomBy zooms around the centre of the view (buttons and keys). */
  const zoomBy = (factor: number, e?: { preventDefault(): void }) => {
    e?.preventDefault();
    setView((v) => zoomAt(v, factor, { x: v.x + W / v.zoom / 2, y: v.y + H / v.zoom / 2 }));
  };
  useEffect(() => {
    const el = svg.current;
    if (!el) return undefined;
    // Wheel (and trackpad pinch, which arrives as ctrl+wheel) zooms around the pointer; shift+wheel pans.
    const wheel = (e: WheelEvent) => {
      e.preventDefault();
      const m = el.getScreenCTM();
      if (!m) return;
      const p = new DOMPoint(e.clientX, e.clientY).matrixTransform(m.inverse());
      if (e.shiftKey && !e.ctrlKey) setView((v) => panBy(v, -e.deltaY, 0));
      else setView((v) => zoomAt(v, Math.exp(-e.deltaY * (e.ctrlKey ? 0.01 : 0.0015)), { x: p.x, y: p.y }));
    };
    el.addEventListener("wheel", wheel, { passive: false });
    return () => el.removeEventListener("wheel", wheel);
  }, []);

  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if ((e.target as HTMLElement).closest("input, textarea, select")) return;
      if (e.key === " ") {
        space.current = e.type === "keydown";
        e.preventDefault();
        return;
      }
      if (e.type !== "keydown") return;
      if ((e.key === "Delete" || e.key === "Backspace") && selection.length > 0) remove();
      const mod = e.ctrlKey || e.metaKey;
      if (mod && e.key === "a") {
        e.preventDefault();
        setSelectedIds(Object.keys(wb.elements));
      }
      if (e.key === "Escape") setSelectedIds([]);
      if (mod && e.key === "d" && selection.length > 0) {
        e.preventDefault();
        addCopies(selection, 24);
      }
      if (mod && e.key === "c") copy();
      if (mod && e.key === "x" && selection.length > 0) {
        copy();
        remove();
      }
      if (mod && e.key === "v") void paste();
      if (mod && e.key === "]" && selection.length > 0) wb.changeAll(toFront(selection, wb.elements).map((element) => ({ op: "update", element })));
      if (mod && e.key === "[" && selection.length > 0) wb.changeAll(toBack(selection, wb.elements).map((element) => ({ op: "update", element })));
      const step = e.shiftKey ? 10 : 1;
      const arrows: Record<string, [number, number]> = { ArrowLeft: [-step, 0], ArrowRight: [step, 0], ArrowUp: [0, -step], ArrowDown: [0, step] };
      if (arrows[e.key] && selection.length > 0) {
        e.preventDefault();
        nudge(...arrows[e.key]!);
      }
      if (e.shiftKey && e.key === "!") setView(fit(Object.values(wb.elements), PAGE));
      if ((e.ctrlKey || e.metaKey) && (e.key === "=" || e.key === "+")) zoomBy(1.25, e);
      if ((e.ctrlKey || e.metaKey) && e.key === "-") zoomBy(0.8, e);
      if ((e.ctrlKey || e.metaKey) && e.key === "0") zoomBy(1 / view.zoom, e);
      if ((e.ctrlKey || e.metaKey) && e.key === "z") e.shiftKey ? wb.redo() : wb.undo();
    };
    window.addEventListener("keydown", key);
    window.addEventListener("keyup", key);
    return () => {
      window.removeEventListener("keydown", key);
      window.removeEventListener("keyup", key);
    };
  });

  const exportAs = async (kind: "svg" | "png") => {
    if (!svg.current) return;
    // S89: the export shows every element, not just what is on screen.
    const b = bounds(Object.values(wb.elements)) ?? { x: 0, y: 0, w: W, h: H };
    const area = { x: b.x - 40, y: b.y - 40, w: Math.max(b.w + 80, 1), h: Math.max(b.h + 80, 1) };
    const copy = svg.current.cloneNode(true) as SVGSVGElement;
    copy.setAttribute("viewBox", `${area.x} ${area.y} ${area.w} ${area.h}`);
    copy.setAttribute("width", String(Math.round(area.w)));
    copy.setAttribute("height", String(Math.round(area.h)));
    copy.querySelector(".wb-background")?.setAttribute("x", String(area.x));
    copy.querySelector(".wb-background")?.setAttribute("y", String(area.y));
    copy.querySelector(".wb-background")?.setAttribute("width", String(area.w));
    copy.querySelector(".wb-background")?.setAttribute("height", String(area.h));
    const markup = new XMLSerializer().serializeToString(copy);
    const svgUrl = URL.createObjectURL(new Blob([markup], { type: "image/svg+xml" }));
    const save = (url: string, ext: string) => {
      const a = document.createElement("a");
      a.href = url;
      a.download = `${wb.board?.title ?? "whiteboard"}.${ext}`;
      a.click();
    };
    if (kind === "svg") return save(svgUrl, "svg");
    const img = new Image();
    img.onload = () => {
      const canvas = document.createElement("canvas");
      canvas.width = Math.round(area.w);
      canvas.height = Math.round(area.h);
      const ctx = canvas.getContext("2d");
      if (!ctx) return;
      ctx.fillStyle = "#ffffff";
      ctx.fillRect(0, 0, canvas.width, canvas.height);
      ctx.drawImage(img, 0, 0);
      save(canvas.toDataURL("image/png"), "png");
    };
    img.src = svgUrl;
  };

  const shown = { ...wb.elements };
  for (const m of moving ?? []) shown[m.id] = m;
  if (draft) shown[draft.id] = draft;
  const shapes = ordered(shown);
  const cursorColor = (u: string) => CURSOR_COLORS[[...u].reduce((n, c) => n + c.charCodeAt(0), 0) % CURSOR_COLORS.length];
  return (
    <section className="wb panel" aria-label={t("Whiteboard {title}", { title: wb.board?.title ?? "" })}>
      <header className="wb-bar">
        <b>{wb.board?.title}</b>
        <div role="toolbar" aria-label={t("Drawing tools")} className="row">
          <button className="ghost" aria-label={t("Add an image")} title={t("Add an image")} onClick={() => fileInput.current?.click()}>🖼</button>
          <input ref={fileInput} type="file" accept="image/png,image/jpeg,image/gif,image/webp" hidden aria-label={t("Image file")}
            onChange={(e) => {
              const f = e.target.files?.[0];
              e.target.value = "";
              if (f) void addImage(f);
            }} />
          {TOOLS.map((x) => (
            <button key={x.tool} className={tool === x.tool ? "active" : "ghost"} aria-pressed={tool === x.tool} aria-label={t(x.label)} title={t(x.label)}
              onClick={() => setTool(x.tool)}>{x.icon}</button>
          ))}
        </div>
        <div className="row" role="group" aria-label={t("Style")}>
          {([1, 2, 3] as const).map((w) => (
            <button key={w} className={(style.stroke || 2) === w ? "active" : "ghost"} aria-pressed={(style.stroke || 2) === w}
              aria-label={t(STROKE_LABELS[w])} title={t(STROKE_LABELS[w])} onClick={() => applyStyle({ stroke: w })}>
              <span className="wb-weight" style={{ height: w === 1 ? 1 : w === 2 ? 3 : 6 }} />
            </button>
          ))}
          <button className={style.fill ? "active" : "ghost"} aria-pressed={!!style.fill} aria-label={t("Fill")} title={t("Fill shapes")} onClick={() => applyStyle({ fill: !style.fill })}>◼</button>
          <button className={style.dashed ? "active" : "ghost"} aria-pressed={!!style.dashed} aria-label={t("Dashed")} title={t("Dashed lines")} onClick={() => applyStyle({ dashed: !style.dashed })}>┄</button>
        </div>
        {selection.some((x) => x.kind === "sticky") && (
          <div className="row" role="radiogroup" aria-label={t("Sticky colour")}>
            {STICKY_COLORS.map((c) => (
              <button key={c} role="radio" aria-checked={selection.every((x) => x.kind !== "sticky" || x.color === c)} aria-label={t("Sticky {color}", { color: c })}
                className="wb-swatch" style={{ background: c }} onClick={() => recolorNotes(c)} />
            ))}
          </div>
        )}
        <div className="row" role="radiogroup" aria-label={t("Color")}>
          {COLORS.map((c) => (
            <button key={c} role="radio" aria-checked={color === c} aria-label={t("Color {color}", { color: c })} className="wb-swatch" style={{ background: c }} onClick={() => recolor(c)} />
          ))}
        </div>
        <div className="row">
          <select aria-label={t("Insert template")} value="" onChange={(e) => e.target.value && insertTemplate(e.target.value as Template)}>
            <option value="">{t("Template…")}</option>
            {TEMPLATES.map((x) => <option key={x.id} value={x.id}>{t(TEMPLATE_LABELS[x.id])}</option>)}
          </select>
          <button className="ghost" onClick={wb.undo} disabled={!wb.canUndo}>{t("Undo")}</button>
          <button className="ghost" onClick={wb.redo} disabled={!wb.canRedo}>{t("Redo")}</button>
          {sel && editable(sel) && <button className="ghost" onClick={editText}>{t("Edit text")}</button>}
          {sel?.kind === "sticky" && sel.text.trim() && !linkedIssue(sel.text) && (projects.data?.length ?? 0) > 0 && (
            <select aria-label={t("Create issue from sticky note")} value="" onChange={(e) => e.target.value && toIssue.mutate({ note: sel, key: e.target.value })}>
              <option value="">{t("Create issue in…")}</option>
              {projects.data?.map((p) => <option key={p.key} value={p.key}>{p.key} — {p.name}</option>)}
            </select>
          )}
          {sel?.kind === "sticky" && linkedIssue(sel.text) && (
            <Link to={`/projects/${linkedIssue(sel.text)!.split("-")[0]}?issue=${linkedIssue(sel.text)}`}>{t("Open {key}", { key: linkedIssue(sel.text)! })}</Link>
          )}
          {selection.length > 1 && <span className="muted">{t("{n} selected", { n: selection.length })}</span>}
          {selection.length > 0 && <button className="ghost" onClick={() => addCopies(selection, 24)} title="Ctrl+D">{t("Duplicate")}</button>}
          {selection.length > 0 && <button className="ghost" onClick={remove}>{t("Delete")}</button>}
          {selection.length > 0 && (
            <>
              <button className="ghost" aria-label={t("Bring to front")} title={t("Bring to front (Ctrl+])")}
                onClick={() => wb.changeAll(toFront(selection, wb.elements).map((element) => ({ op: "update", element })))}>⬆</button>
              <button className="ghost" aria-label={t("Send to back")} title={t("Send to back (Ctrl+[)")}
                onClick={() => wb.changeAll(toBack(selection, wb.elements).map((element) => ({ op: "update", element })))}>⬇</button>
            </>
          )}
          <button className="ghost" onClick={() => void exportAs("svg")}>{t("Export SVG")}</button>
          <button className="ghost" onClick={() => void exportAs("png")}>{t("Export PNG")}</button>
          {onClose && <button className="ghost" aria-label={t("Close whiteboard")} onClick={onClose}>✕</button>}
        </div>
      </header>
      <div className="wb-stage" onDragOver={(e) => e.preventDefault()}
        onDrop={(e) => {
          const f = [...e.dataTransfer.files].find((x) => x.type.startsWith("image/"));
          if (f) {
            e.preventDefault();
            void addImage(f);
          }
        }}>
      <svg ref={svg} role="img" aria-label={t("Canvas")} viewBox={viewBox(view, PAGE)} className={`wb-canvas tool-${tool}`} xmlns="http://www.w3.org/2000/svg"
        onPointerDown={down} onPointerMove={move} onPointerUp={up}
        onDoubleClick={() => editText()}>
        <defs>
          <marker id="wb-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="8" markerHeight="8" orient="auto-start-reverse">
            <path d="M 0 0 L 10 5 L 0 10 z" fill="context-stroke" />
          </marker>
          <pattern id="wb-dots" width="24" height="24" patternUnits="userSpaceOnUse">
            <circle cx="1" cy="1" r="1" fill="#d4d4d8" />
          </pattern>
        </defs>
        <rect className="wb-background" x={view.x} y={view.y} width={W / view.zoom} height={H / view.zoom} fill="#ffffff" />
        <rect className="wb-grid" x={view.x} y={view.y} width={W / view.zoom} height={H / view.zoom} fill="url(#wb-dots)" pointerEvents="none" />
        {shapes.map((e) => (
          <g key={e.id} opacity={erasing.includes(e.id) ? 0.25 : undefined}><Shape e={e} selected={selected.includes(e.id)} board={id} /></g>
        ))}
        {sel && !editing && <Handles e={sel} zoom={view.zoom} />}
        {selection.length > 1 && (() => {
          const b = bounds(selection)!;
          return <rect x={b.x - 6} y={b.y - 6} width={b.w + 12} height={b.h + 12} fill="none" stroke="#2563eb" strokeDasharray="6 4" pointerEvents="none" aria-label={t("Selection")} />;
        })()}
        {marquee && <rect x={marquee.x} y={marquee.y} width={marquee.w} height={marquee.h} fill="#2563eb22" stroke="#2563eb" pointerEvents="none" />}
        {shapes.filter((e) => counts.has(e.id)).map((e) => (
          <g key={`c-${e.id}`} pointerEvents="none" aria-label={t("{n} comments on an element", { n: counts.get(e.id) ?? 0 })}>
            <circle cx={e.x + Math.max(e.w, 0) + 6} cy={e.y - 6} r={12} fill="#2563eb" />
            <text x={e.x + Math.max(e.w, 0) + 6} y={e.y - 1} fontSize={13} fill="#fff" textAnchor="middle">{counts.get(e.id)}</text>
          </g>
        ))}
        {Object.entries(wb.cursors).filter(([, c]) => Date.now() - c.at < 10_000).map(([u, c]) => (
          <g key={u} transform={`translate(${c.x} ${c.y})`} pointerEvents="none" aria-label={t("{name}'s cursor", { name: names.get(u) ?? t("Someone") })}>
            <path d="M0 0 L0 18 L5 13 L12 13 Z" fill={cursorColor(u)} />
            <text x={14} y={22} fontSize={14} fill={cursorColor(u)}>{names.get(u) ?? t("Someone")}</text>
          </g>
        ))}
      </svg>
      <div className="wb-zoom" role="group" aria-label={t("Zoom")}>
        <button className="ghost" aria-label={t("Zoom out")} onClick={() => zoomBy(0.8)}>−</button>
        <button className="ghost" aria-label={t("Reset zoom")} title={t("Reset to 100%")} onClick={() => zoomBy(1 / view.zoom)}>{Math.round(view.zoom * 100)}%</button>
        <button className="ghost" aria-label={t("Zoom in")} onClick={() => zoomBy(1.25)}>+</button>
        <button className="ghost" aria-label={t("Fit to content")} title={t("Fit (Shift+1)")} onClick={() => setView(fit(Object.values(wb.elements), PAGE))}>⤢</button>
      </div>
      {editing && <TextEditor key={editing.element.id} e={editing.element} view={view} onDone={finishEditing} />}
      </div>
      {sel && !draft && !moving && <ElementComments board={id} element={sel.id} me={me} names={names} />}
      {wb.error && <p role="status" className="muted">{wb.error}</p>}
      {imageError && <p role="alert" className="error">{imageError}</p>}
      {toIssue.error && <p role="alert" className="error">{toIssue.error.message}</p>}
    </section>
  );
}
