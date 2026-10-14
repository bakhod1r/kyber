import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, type PointerEvent, useEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { issueUseCases } from "../../issue/application";
import { projectUseCases } from "../../project/application";
import {
  bounds, box, COLORS, commentCounts, duplicate, type Element, fit, fromClipboard, type Handle, HANDLES, hit, HOME, type Kind, linkedIssue, moveBy, moveEnd, newId,
  onScreen, onWhiteboardComments, ordered, panBy, resize, toClipboard, within,
  type Point, STICKY, stickyToIssue, useWhiteboard, type View, viewBox, whiteboardUseCases, zoomAt,
} from "../application";

const W = 1600;
const H = 900;
const PAGE = { w: W, h: H };
type Tool = "select" | "hand" | Kind;
const TOOLS: { tool: Tool; label: string; icon: string }[] = [
  { tool: "select", label: "Select", icon: "↖" },
  { tool: "hand", label: "Hand", icon: "✋" },
  { tool: "rect", label: "Rectangle", icon: "▭" },
  { tool: "ellipse", label: "Ellipse", icon: "◯" },
  { tool: "arrow", label: "Arrow", icon: "→" },
  { tool: "line", label: "Line", icon: "／" },
  { tool: "freehand", label: "Pen", icon: "✎" },
  { tool: "text", label: "Text", icon: "T" },
  { tool: "sticky", label: "Sticky note", icon: "🗒" },
];
const CURSOR_COLORS = ["#e11d48", "#7c3aed", "#0891b2", "#ca8a04", "#059669"];

/**
 * The whiteboards of a conversation (S59): pick or create one, then draw together — shapes, arrows, pen,
 * text and sticky notes, with everyone's changes and cursors live. Slotted into the chat by the app shell.
 */
export function Whiteboards({ conversationId, me, names }: { conversationId: string; me: string; names: Map<string, string> }) {
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
  if (!shown) return <button className="ghost" onClick={() => setShown(true)}>Whiteboards</button>;
  if (open) return <Board id={open} me={me} names={names} onClose={() => setOpen(null)} />;
  const submit = (e: FormEvent) => {
    e.preventDefault();
    create.mutate();
  };
  return (
    <section className="wb-list panel" aria-label="Whiteboards">
      <header className="row">
        <h2>Whiteboards</h2>
        <button className="ghost" aria-label="Close whiteboards" onClick={() => setShown(false)}>✕</button>
      </header>
      <ul>
        {list.data?.length === 0 && <li className="muted">No whiteboards yet.</li>}
        {list.data?.map((b) => (
          <li key={b.id}><button className="link" onClick={() => setOpen(b.id)}>{b.title}</button> <span className="muted">by {names.get(b.created_by) ?? "someone"}</span></li>
        ))}
      </ul>
      <form onSubmit={submit} className="row">
        <input aria-label="Whiteboard title" placeholder="Architecture" value={title} onChange={(e) => setTitle(e.target.value)} required />
        <button>New whiteboard</button>
      </form>
      {create.error && <p role="alert" className="error">{create.error.message}</p>}
    </section>
  );
}

function Shape({ e, selected }: { e: Element; selected: boolean }) {
  const stroke = e.color || "#1f2937";
  const common = { "data-id": e.id, stroke, strokeWidth: selected ? 3 : 2, fill: "none" } as const;
  const pts = e.points.map((p) => `${e.x + p.x},${e.y + p.y}`).join(" ");
  switch (e.kind) {
    case "rect":
      return <g><rect {...common} x={e.x} y={e.y} width={e.w} height={e.h} rx={6} /><Label e={e} /></g>;
    case "ellipse":
      return <g><ellipse {...common} cx={e.x + e.w / 2} cy={e.y + e.h / 2} rx={e.w / 2} ry={e.h / 2} /><Label e={e} /></g>;
    case "line":
      return <polyline {...common} points={pts} />;
    case "arrow":
      return <polyline {...common} points={pts} markerEnd="url(#wb-arrow)" />;
    case "freehand":
      return <polyline {...common} points={pts} strokeLinecap="round" strokeLinejoin="round" />;
    case "sticky":
      return <g><rect {...common} x={e.x} y={e.y} width={e.w} height={e.h} fill={STICKY} stroke={selected ? stroke : "#d4b84a"} /><Label e={e} /></g>;
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
  const r = 6 / zoom;
  if (e.kind === "line" || e.kind === "arrow") {
    return (
      <g>
        {[0, e.points.length - 1].map((i) => (
          <circle key={i} data-handle={`end-${i}`} aria-label={i === 0 ? "Line start" : "Line end"} className="wb-handle" cx={e.x + e.points[i]!.x} cy={e.y + e.points[i]!.y} r={r} />
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
        <rect key={h} data-handle={h} aria-label={`Resize ${h}`} className={`wb-handle wb-handle-${h}`}
          x={e.x + at[h][0] * e.w - r} y={e.y + at[h][1] * e.h - r} width={2 * r} height={2 * r} />
      ))}
    </g>
  );
}

/** Types an element's text in place over the canvas: Enter (Shift+Enter for a new line) or leaving saves, Escape cancels. */
function TextEditor({ e, view, onDone }: { e: Element; view: View; onDone: (text: string | null) => void }) {
  const done = useRef(false);
  const finish = (text: string | null) => {
    if (done.current) return;
    done.current = true;
    onDone(text);
  };
  const pct = (v: number, of: number) => `${(v / of) * 100}%`;
  return (
    <textarea className={`wb-editor ${e.kind === "sticky" ? "sticky" : ""}`} aria-label={e.kind === "sticky" ? "Sticky note text" : "Text"} autoFocus
      defaultValue={e.text} placeholder={e.kind === "sticky" ? "Sticky note" : "Type…"}
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
  const qc = useQueryClient();
  const key = ["whiteboard", "comments", board];
  const list = useQuery({ queryKey: key, queryFn: () => whiteboardUseCases.comments(board) });
  const [body, setBody] = useState("");
  const done = () => qc.invalidateQueries({ queryKey: key });
  const add = useMutation({ mutationFn: () => whiteboardUseCases.comment(board, element, body), onSuccess: () => { setBody(""); void done(); } });
  const remove = useMutation({ mutationFn: (id: string) => whiteboardUseCases.removeComment(id), onSuccess: done });
  const mine = (list.data ?? []).filter((c) => c.element_id === element);
  return (
    <aside className="wb-comments panel" aria-label="Element comments">
      <h3>💬 Comments</h3>
      {mine.length === 0 && <p className="muted">No comments yet.</p>}
      <ul>
        {mine.map((c) => (
          <li key={c.id}>
            <b>{c.author_id === me ? "You" : (names.get(c.author_id) ?? "Someone")}</b> {c.body}{" "}
            {c.author_id === me && <button className="ghost" onClick={() => remove.mutate(c.id)}>Remove</button>}
          </li>
        ))}
      </ul>
      <form className="row" onSubmit={(e) => { e.preventDefault(); add.mutate(); }}>
        <input aria-label="Comment" value={body} onChange={(e) => setBody(e.target.value)} placeholder="Comment… (@email to mention)" maxLength={2000} />
        <button disabled={!body.trim()}>Comment</button>
      </form>
    </aside>
  );
}

function Board({ id, me, names, onClose }: { id: string; me: string; names: Map<string, string>; onClose?: () => void }) {
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
  /** S90: the selected elements (Shift+click or a marquee selects several). */
  const [selected, setSelectedIds] = useState<string[]>([]);
  const setSelected = (id: string | null) => setSelectedIds(id ? [id] : []);
  /** S89: the part of the infinite board on screen; pan holds a drag of the board in progress. */
  const [view, setView] = useState<View>(HOME);
  const pan = useRef<{ x: number; y: number; from: View } | null>(null);
  const space = useRef(false);
  const [draft, setDraft] = useState<Element | null>(null); // the shape being drawn
  /** What a drag in the select tool does: move the selection, resize a box, move a line end, or draw a marquee. */
  const drag = useRef<
    | { mode: "draw"; start: Point; origin: Element }
    | { mode: "move"; start: Point; origins: Element[] }
    | { mode: "resize"; start: Point; origin: Element; handle: Handle }
    | { mode: "end"; origin: Element; index: number }
    | { mode: "marquee"; start: Point; add: string[] }
    | null
  >(null);
  const [moving, setMoving] = useState<Element[] | null>(null); // elements being moved or reshaped, before they are sent
  const [marquee, setMarquee] = useState<{ x: number; y: number; w: number; h: number } | null>(null);
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
    const base = { id: newId(), kind: tool, x: p.x, y: p.y, w: 0, h: 0, points: [] as Point[], text: "", color, version: 0 };
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
      const area = marquee;
      setMarquee(null);
      if (area && (area.w > 3 || area.h > 3)) setSelectedIds([...new Set([...d.add, ...within(Object.values(wb.elements), area).map((x) => x.id)])]);
      return;
    }
    if (d && d.mode !== "draw") {
      const changed = (moving ?? []).filter((m) => {
        const o = wb.elements[m.id];
        return o && (o.x !== m.x || o.y !== m.y || o.w !== m.w || o.h !== m.h || o.points !== m.points);
      });
      wb.changeAll(changed.map((element) => ({ op: "update", element })));
      setMoving(null);
      return;
    }
    if (d && draft && (draft.points.length > 1 || draft.w > 3 || draft.h > 3)) wb.change({ op: "add", element: draft });
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
    <section className="wb panel" aria-label={`Whiteboard ${wb.board?.title ?? ""}`}>
      <header className="wb-bar">
        <b>{wb.board?.title}</b>
        <div role="toolbar" aria-label="Drawing tools" className="row">
          {TOOLS.map((t) => (
            <button key={t.tool} className={tool === t.tool ? "active" : "ghost"} aria-pressed={tool === t.tool} aria-label={t.label} title={t.label}
              onClick={() => setTool(t.tool)}>{t.icon}</button>
          ))}
        </div>
        <div className="row" role="radiogroup" aria-label="Color">
          {COLORS.map((c) => (
            <button key={c} role="radio" aria-checked={color === c} aria-label={`Color ${c}`} className="wb-swatch" style={{ background: c }} onClick={() => recolor(c)} />
          ))}
        </div>
        <div className="row">
          <button className="ghost" onClick={wb.undo} disabled={!wb.canUndo}>Undo</button>
          <button className="ghost" onClick={wb.redo} disabled={!wb.canRedo}>Redo</button>
          {sel && editable(sel) && <button className="ghost" onClick={editText}>Edit text</button>}
          {sel?.kind === "sticky" && sel.text.trim() && !linkedIssue(sel.text) && (projects.data?.length ?? 0) > 0 && (
            <select aria-label="Create issue from sticky note" value="" onChange={(e) => e.target.value && toIssue.mutate({ note: sel, key: e.target.value })}>
              <option value="">Create issue in…</option>
              {projects.data?.map((p) => <option key={p.key} value={p.key}>{p.key} — {p.name}</option>)}
            </select>
          )}
          {sel?.kind === "sticky" && linkedIssue(sel.text) && (
            <Link to={`/projects/${linkedIssue(sel.text)!.split("-")[0]}?issue=${linkedIssue(sel.text)}`}>Open {linkedIssue(sel.text)}</Link>
          )}
          {selection.length > 1 && <span className="muted">{selection.length} selected</span>}
          {selection.length > 0 && <button className="ghost" onClick={() => addCopies(selection, 24)} title="Ctrl+D">Duplicate</button>}
          {selection.length > 0 && <button className="ghost" onClick={remove}>Delete</button>}
          <button className="ghost" onClick={() => void exportAs("svg")}>Export SVG</button>
          <button className="ghost" onClick={() => void exportAs("png")}>Export PNG</button>
          {onClose && <button className="ghost" aria-label="Close whiteboard" onClick={onClose}>✕</button>}
        </div>
      </header>
      <div className="wb-stage">
      <svg ref={svg} role="img" aria-label="Canvas" viewBox={viewBox(view, PAGE)} className={`wb-canvas tool-${tool}`} xmlns="http://www.w3.org/2000/svg"
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
        {shapes.map((e) => <Shape key={e.id} e={e} selected={selected.includes(e.id)} />)}
        {sel && !editing && <Handles e={sel} zoom={view.zoom} />}
        {selection.length > 1 && (() => {
          const b = bounds(selection)!;
          return <rect x={b.x - 6} y={b.y - 6} width={b.w + 12} height={b.h + 12} fill="none" stroke="#2563eb" strokeDasharray="6 4" pointerEvents="none" aria-label="Selection" />;
        })()}
        {marquee && <rect x={marquee.x} y={marquee.y} width={marquee.w} height={marquee.h} fill="#2563eb22" stroke="#2563eb" pointerEvents="none" />}
        {shapes.filter((e) => counts.has(e.id)).map((e) => (
          <g key={`c-${e.id}`} pointerEvents="none" aria-label={`${counts.get(e.id)} comments on an element`}>
            <circle cx={e.x + Math.max(e.w, 0) + 6} cy={e.y - 6} r={12} fill="#2563eb" />
            <text x={e.x + Math.max(e.w, 0) + 6} y={e.y - 1} fontSize={13} fill="#fff" textAnchor="middle">{counts.get(e.id)}</text>
          </g>
        ))}
        {Object.entries(wb.cursors).filter(([, c]) => Date.now() - c.at < 10_000).map(([u, c]) => (
          <g key={u} transform={`translate(${c.x} ${c.y})`} pointerEvents="none" aria-label={`${names.get(u) ?? "Someone"}'s cursor`}>
            <path d="M0 0 L0 18 L5 13 L12 13 Z" fill={cursorColor(u)} />
            <text x={14} y={22} fontSize={14} fill={cursorColor(u)}>{names.get(u) ?? "Someone"}</text>
          </g>
        ))}
      </svg>
      <div className="wb-zoom" role="group" aria-label="Zoom">
        <button className="ghost" aria-label="Zoom out" onClick={() => zoomBy(0.8)}>−</button>
        <button className="ghost" aria-label="Reset zoom" title="Reset to 100%" onClick={() => zoomBy(1 / view.zoom)}>{Math.round(view.zoom * 100)}%</button>
        <button className="ghost" aria-label="Zoom in" onClick={() => zoomBy(1.25)}>+</button>
        <button className="ghost" aria-label="Fit to content" title="Fit (Shift+1)" onClick={() => setView(fit(Object.values(wb.elements), PAGE))}>⤢</button>
      </div>
      {editing && <TextEditor key={editing.element.id} e={editing.element} view={view} onDone={finishEditing} />}
      </div>
      {sel && !draft && !moving && <ElementComments board={id} element={sel.id} me={me} names={names} />}
      {wb.error && <p role="status" className="muted">{wb.error}</p>}
      {toIssue.error && <p role="alert" className="error">{toIssue.error.message}</p>}
    </section>
  );
}
