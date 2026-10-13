import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { type FormEvent, type PointerEvent, useEffect, useRef, useState } from "react";
import {
  box, COLORS, type Element, hit, type Kind, newId, ordered, type Point, STICKY, useWhiteboard, whiteboardUseCases,
} from "../application";

const W = 1600;
const H = 900;
type Tool = "select" | Kind;
const TOOLS: { tool: Tool; label: string; icon: string }[] = [
  { tool: "select", label: "Select", icon: "↖" },
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

/** One whiteboard to draw on (also what a huddle shows when it is presented). */
export function WhiteboardBoard(props: { id: string; me: string; names: Map<string, string>; onClose?: () => void }) {
  return <Board {...props} />;
}

function Board({ id, me, names, onClose }: { id: string; me: string; names: Map<string, string>; onClose?: () => void }) {
  const wb = useWhiteboard(id, me);
  const svg = useRef<SVGSVGElement>(null);
  const [tool, setTool] = useState<Tool>("select");
  const [color, setColor] = useState(COLORS[0]!);
  const [selected, setSelected] = useState<string | null>(null);
  const [draft, setDraft] = useState<Element | null>(null); // the shape being drawn or dragged
  const drag = useRef<{ start: Point; origin: Element } | null>(null);
  const sel = selected ? (draft?.id === selected ? draft : wb.elements[selected]) : undefined;

  const at = (e: PointerEvent): Point => {
    const m = svg.current?.getScreenCTM();
    if (!m) return { x: 0, y: 0 };
    const p = new DOMPoint(e.clientX, e.clientY).matrixTransform(m.inverse());
    return { x: Math.round(p.x), y: Math.round(p.y) };
  };

  const down = (e: PointerEvent<SVGSVGElement>) => {
    const p = at(e);
    svg.current?.setPointerCapture?.(e.pointerId);
    if (tool === "select") {
      const target = [...ordered(wb.elements)].reverse().find((x) => hit(x, p));
      setSelected(target?.id ?? null);
      if (target) drag.current = { start: p, origin: target };
      return;
    }
    const base = { id: newId(), kind: tool, x: p.x, y: p.y, w: 0, h: 0, points: [] as Point[], text: "", color, version: 0 };
    if (tool === "text" || tool === "sticky") {
      const text = window.prompt(tool === "sticky" ? "Sticky note" : "Text") ?? "";
      if (!text.trim()) return;
      const el: Element = { ...base, text, w: tool === "sticky" ? 180 : Math.max(60, text.length * 10), h: tool === "sticky" ? 120 : 34, color: tool === "sticky" ? STICKY : color };
      wb.change({ op: "add", element: el });
      setSelected(el.id);
      setTool("select");
      return;
    }
    drag.current = { start: p, origin: base };
    setDraft({ ...base, points: tool === "freehand" || tool === "line" || tool === "arrow" ? [{ x: 0, y: 0 }] : [] });
  };

  const move = (e: PointerEvent<SVGSVGElement>) => {
    const p = at(e);
    wb.moveCursor(p.x, p.y);
    const d = drag.current;
    if (!d) return;
    const o = d.origin;
    if (tool === "select") {
      setDraft({ ...o, x: o.x + p.x - d.start.x, y: o.y + p.y - d.start.y });
    } else if (tool === "freehand") {
      setDraft((cur) => (cur ? { ...cur, points: [...cur.points, { x: p.x - o.x, y: p.y - o.y }] } : cur));
    } else if (tool === "line" || tool === "arrow") {
      setDraft({ ...o, points: [{ x: 0, y: 0 }, { x: p.x - o.x, y: p.y - o.y }] });
    } else {
      setDraft({ ...o, ...box(d.start.x, d.start.y, p.x, p.y) });
    }
  };

  const up = () => {
    const d = drag.current;
    drag.current = null;
    if (!d || !draft) return setDraft(null);
    if (tool === "select") {
      if (draft.x !== d.origin.x || draft.y !== d.origin.y) wb.change({ op: "update", element: draft });
    } else if (draft.points.length > 1 || draft.w > 3 || draft.h > 3) {
      wb.change({ op: "add", element: draft });
    }
    setDraft(null);
  };

  const remove = () => {
    if (!sel) return;
    wb.change({ op: "delete", element: sel });
    setSelected(null);
  };
  const editText = () => {
    if (!sel) return;
    const text = window.prompt("Text", sel.text);
    if (text !== null) wb.change({ op: "update", element: { ...sel, text } });
  };
  const recolor = (c: string) => {
    setColor(c);
    if (sel && sel.kind !== "sticky") wb.change({ op: "update", element: { ...sel, color: c } });
  };

  useEffect(() => {
    const key = (e: KeyboardEvent) => {
      if ((e.target as HTMLElement).closest("input, textarea")) return;
      if ((e.key === "Delete" || e.key === "Backspace") && selected) remove();
      if ((e.ctrlKey || e.metaKey) && e.key === "z") e.shiftKey ? wb.redo() : wb.undo();
    };
    window.addEventListener("keydown", key);
    return () => window.removeEventListener("keydown", key);
  });

  const exportAs = async (kind: "svg" | "png") => {
    if (!svg.current) return;
    const markup = new XMLSerializer().serializeToString(svg.current);
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
      canvas.width = W;
      canvas.height = H;
      const ctx = canvas.getContext("2d");
      if (!ctx) return;
      ctx.fillStyle = "#ffffff";
      ctx.fillRect(0, 0, W, H);
      ctx.drawImage(img, 0, 0);
      save(canvas.toDataURL("image/png"), "png");
    };
    img.src = svgUrl;
  };

  const shapes = ordered(draft ? { ...wb.elements, [draft.id]: draft } : wb.elements);
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
          {sel && (sel.kind === "text" || sel.kind === "sticky" || sel.kind === "rect" || sel.kind === "ellipse") && <button className="ghost" onClick={editText}>Edit text</button>}
          {sel && <button className="ghost" onClick={remove}>Delete</button>}
          <button className="ghost" onClick={() => void exportAs("svg")}>Export SVG</button>
          <button className="ghost" onClick={() => void exportAs("png")}>Export PNG</button>
          {onClose && <button className="ghost" aria-label="Close whiteboard" onClick={onClose}>✕</button>}
        </div>
      </header>
      <svg ref={svg} role="img" aria-label="Canvas" viewBox={`0 0 ${W} ${H}`} className={`wb-canvas tool-${tool}`} xmlns="http://www.w3.org/2000/svg"
        onPointerDown={down} onPointerMove={move} onPointerUp={up}>
        <defs>
          <marker id="wb-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="8" markerHeight="8" orient="auto-start-reverse">
            <path d="M 0 0 L 10 5 L 0 10 z" fill="context-stroke" />
          </marker>
        </defs>
        <rect width={W} height={H} fill="#ffffff" />
        {shapes.map((e) => <Shape key={e.id} e={e} selected={e.id === selected} />)}
        {Object.entries(wb.cursors).filter(([, c]) => Date.now() - c.at < 10_000).map(([u, c]) => (
          <g key={u} transform={`translate(${c.x} ${c.y})`} pointerEvents="none" aria-label={`${names.get(u) ?? "Someone"}'s cursor`}>
            <path d="M0 0 L0 18 L5 13 L12 13 Z" fill={cursorColor(u)} />
            <text x={14} y={22} fontSize={14} fill={cursorColor(u)}>{names.get(u) ?? "Someone"}</text>
          </g>
        ))}
      </svg>
      {wb.error && <p role="status" className="muted">{wb.error}</p>}
    </section>
  );
}
