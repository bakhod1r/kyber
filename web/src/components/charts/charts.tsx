// Chart primitives built to the dataviz method: thin marks, 4px rounded data-ends,
// hairline solid grid, legend for >= 2 series, crosshair tooltip on lines, and a
// table view for every chart. Colors come from validated CSS tokens (--viz-*).
import { type PointerEvent, type ReactNode, useId, useState } from "react";

export type Slot = 1 | 2 | "muted";
const stroke = (slot: Slot) => (slot === "muted" ? "var(--viz-muted)" : `var(--viz-${slot})`);

export const fmt = (n: number) => (Number.isInteger(n) ? n.toLocaleString() : n.toFixed(1));

/** Smallest "clean" axis maximum >= v. */
export function niceMax(v: number): number {
  if (v <= 0) return 1;
  const p = 10 ** Math.floor(Math.log10(v));
  for (const m of [1, 1.5, 2, 2.5, 3, 4, 5, 6, 8, 10]) if (m * p >= v) return m * p;
  return 10 * p;
}

const shortDate = (d: Date) => d.toLocaleDateString(undefined, { month: "short", day: "numeric", timeZone: "UTC" });

function Figure({ title, caption, legend, table, children }: { title: string; caption?: ReactNode; legend?: ReactNode; table: ReactNode; children: ReactNode }) {
  const id = useId();
  return (
    <figure className="chart" aria-labelledby={id}>
      <h3 id={id}>{title}</h3>
      {legend}
      {children}
      {caption && <figcaption className="muted">{caption}</figcaption>}
      <details className="chart-table">
        <summary>Show data table</summary>
        {table}
      </details>
    </figure>
  );
}

function Legend({ items }: { items: { name: string; slot: Slot }[] }) {
  return (
    <ul className="legend" aria-label="Legend">
      {items.map((s) => (
        <li key={s.name}>
          <svg width="16" height="8" aria-hidden="true">
            <line x1="1" y1="4" x2="15" y2="4" stroke={stroke(s.slot)} strokeWidth="2" strokeLinecap="round" />
          </svg>
          {s.name}
        </li>
      ))}
    </ul>
  );
}

// Horizontal bar with a square baseline end and a 4px rounded data end.
function hbar(x: number, y: number, w: number, h: number) {
  const r = Math.min(4, w / 2, h / 2);
  return `M${x},${y} H${x + w - r} Q${x + w},${y} ${x + w},${y + r} V${y + h - r} Q${x + w},${y + h} ${x + w - r},${y + h} H${x} Z`;
}
// Vertical column: square at the baseline (bottom), rounded at the top.
function vbar(x: number, base: number, w: number, h: number) {
  const r = Math.min(4, w / 2, h);
  const top = base - h;
  return `M${x},${base} V${top + r} Q${x},${top} ${x + r},${top} H${x + w - r} Q${x + w},${top} ${x + w},${top + r} V${base} Z`;
}

export type BarItem = { label: string; value: number; extra?: number };

/** One series, so no legend: the category labels carry identity. */
export function BarList({ title, items, valueLabel = "Issues", extraLabel, caption }: { title: string; items: BarItem[]; valueLabel?: string; extraLabel?: string; caption?: ReactNode }) {
  const max = niceMax(Math.max(0, ...items.map((i) => i.value)));
  const W = 320, rowH = 32, barH = 16;
  return (
    <Figure
      title={title}
      caption={caption}
      table={
        <table>
          <thead>
            <tr><th>Category</th><th>{valueLabel}</th>{extraLabel && <th>{extraLabel}</th>}</tr>
          </thead>
          <tbody>
            {items.map((i) => (
              <tr key={i.label}><td>{i.label}</td><td>{fmt(i.value)}</td>{extraLabel && <td>{fmt(i.extra ?? 0)}</td>}</tr>
            ))}
          </tbody>
        </table>
      }
    >
      {items.length === 0 ? (
        <p className="muted">No issues yet.</p>
      ) : (
        <div className="barlist">
          {items.map((i) => {
            const w = Math.max(i.value > 0 ? 2 : 0, (i.value / max) * W);
            return (
              <div className="barlist-row" key={i.label} title={`${i.label}: ${fmt(i.value)}${extraLabel ? ` · ${fmt(i.extra ?? 0)} ${extraLabel.toLowerCase()}` : ""}`}>
                <span className="barlist-label">{i.label}</span>
                <svg viewBox={`0 0 ${W + 48} ${rowH}`} preserveAspectRatio="xMinYMid meet" aria-hidden="true">
                  <path data-bar data-width={w.toFixed(2)} d={hbar(0, (rowH - barH) / 2, w, barH)} fill="var(--viz-1)" />
                  <text x={w + 6} y={rowH / 2} dominantBaseline="middle" className="viz-value">{fmt(i.value)}</text>
                </svg>
              </div>
            );
          })}
        </div>
      )}
    </Figure>
  );
}

export type Series = { name: string; slot: Slot; points: { x: Date; y: number }[]; step?: boolean };

const VW = 800, VH = 260, M = { top: 12, right: 44, bottom: 28, left: 40 };

/** Line chart with a crosshair tooltip. Step series hold their value until the next point. */
export function LineChart({ title, series: raw, caption, xMax, yLabel = "Value" }: { title: string; series: Series[]; caption?: ReactNode; xMax?: Date; yLabel?: string }) {
  const [hover, setHover] = useState<number | null>(null);
  // Drop malformed points rather than failing the whole chart.
  const series = raw.map((s) => ({ ...s, points: s.points.filter((p) => !Number.isNaN(p.x.getTime()) && Number.isFinite(p.y)) }));
  const xs = Array.from(new Set(series.flatMap((s) => s.points.map((p) => p.x.getTime())))).sort((a, b) => a - b);
  const x0 = xs[0] ?? 0;
  const x1 = Math.max(xs[xs.length - 1] ?? 1, xMax && !Number.isNaN(xMax.getTime()) ? xMax.getTime() : 0, x0 + 1);
  const yMax = niceMax(Math.max(0, ...series.flatMap((s) => s.points.map((p) => p.y))));
  const pw = VW - M.left - M.right, ph = VH - M.top - M.bottom;
  const sx = (t: number) => M.left + ((t - x0) / (x1 - x0)) * pw;
  const sy = (v: number) => M.top + ph - (v / yMax) * ph;

  const valueAt = (s: Series, t: number): number | undefined => {
    const pts = s.points;
    if (pts.length === 0 || t < pts[0]!.x.getTime()) return undefined;
    let i = 0;
    while (i + 1 < pts.length && pts[i + 1]!.x.getTime() <= t) i++;
    const a = pts[i]!, b = pts[i + 1];
    if (s.step || !b) return a.y;
    const f = (t - a.x.getTime()) / (b.x.getTime() - a.x.getTime());
    return a.y + f * (b.y - a.y);
  };

  const path = (s: Series) =>
    s.points
      .map((p, i) => {
        const X = sx(p.x.getTime()), Y = sy(p.y);
        if (i === 0) return `M${X},${Y}`;
        return s.step ? `H${X} V${Y}` : `L${X},${Y}`;
      })
      .join(" ");

  function onMove(e: PointerEvent<SVGRectElement>) {
    const r = e.currentTarget.getBoundingClientRect();
    const vx = ((e.clientX - r.left) / Math.max(1, r.width)) * VW;
    let best = 0;
    xs.forEach((t, i) => {
      if (Math.abs(sx(t) - vx) < Math.abs(sx(xs[best]!) - vx)) best = i;
    });
    setHover(xs.length ? best : null);
  }

  const ht = hover === null ? null : xs[hover]!;
  const ticks = [0, yMax / 2, yMax];
  const xTicks = [x0, (x0 + x1) / 2, x1];

  return (
    <Figure
      title={title}
      caption={caption}
      legend={series.length >= 2 ? <Legend items={series} /> : undefined}
      table={
        <table>
          <thead>
            <tr><th>Date</th>{series.map((s) => <th key={s.name}>{s.name}</th>)}</tr>
          </thead>
          <tbody>
            {xs.map((t) => (
              <tr key={t}>
                <td>{new Date(t).toISOString().replace("T", " ").slice(0, 16)}</td>
                {series.map((s) => {
                  const v = valueAt(s, t);
                  return <td key={s.name}>{v === undefined ? "–" : fmt(Math.round(v * 10) / 10)}</td>;
                })}
              </tr>
            ))}
          </tbody>
        </table>
      }
    >
      <div className="plot-wrap">
        <svg viewBox={`0 0 ${VW} ${VH}`} className="plot" role="img" aria-label={`${title}: ${yLabel} over time`}>
          {ticks.map((v) => (
            <g key={v}>
              <line x1={M.left} x2={VW - M.right} y1={sy(v)} y2={sy(v)} className="grid" />
              <text x={M.left - 6} y={sy(v)} textAnchor="end" dominantBaseline="middle" className="axis">{fmt(v)}</text>
            </g>
          ))}
          {xTicks.map((t, i) => (
            <text key={i} x={sx(t)} y={VH - 8} textAnchor={i === 0 ? "start" : i === 2 ? "end" : "middle"} className="axis">
              {shortDate(new Date(t))}
            </text>
          ))}
          {series.map((s) => (
            <path key={s.name} d={path(s)} fill="none" stroke={stroke(s.slot)} strokeWidth={2} strokeLinejoin="round" strokeLinecap="round" />
          ))}
          {series.map((s) => {
            const last = s.points[s.points.length - 1];
            if (!last) return null;
            return (
              <g key={s.name}>
                <circle cx={sx(last.x.getTime())} cy={sy(last.y)} r={4} fill={stroke(s.slot)} stroke="var(--viz-surface)" strokeWidth={2} />
                <text x={sx(last.x.getTime()) + 7} y={sy(last.y)} dominantBaseline="middle" className="viz-value">{fmt(Math.round(last.y * 10) / 10)}</text>
              </g>
            );
          })}
          {ht !== null && <line x1={sx(ht)} x2={sx(ht)} y1={M.top} y2={M.top + ph} className="crosshair" />}
          <rect data-plot x={0} y={0} width={VW} height={VH} fill="transparent" onPointerMove={onMove} onPointerLeave={() => setHover(null)} />
        </svg>
        {ht !== null && (
          <div role="status" className="tooltip" style={{ left: `${(sx(ht) / VW) * 100}%` }}>
            <div className="muted">{new Date(ht).toISOString().slice(0, 10)}</div>
            {series.map((s) => {
              const v = valueAt(s, ht);
              return (
                <div key={s.name} className="tip-row">
                  <svg width="12" height="8" aria-hidden="true"><line x1="1" y1="4" x2="11" y2="4" stroke={stroke(s.slot)} strokeWidth="2" /></svg>
                  <strong>{v === undefined ? "–" : fmt(Math.round(v * 10) / 10)}</strong> <span>{s.name}</span>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </Figure>
  );
}

/** Grouped columns (two series), e.g. committed vs completed per sprint. */
export function GroupedBars({ title, categories, series, unit = "points" }: { title: string; categories: string[]; series: { name: string; slot: Slot; values: number[] }[]; unit?: string }) {
  const max = niceMax(Math.max(0, ...series.flatMap((s) => s.values)));
  const H = 220, base = H - 24, top = 12, colW = 22, gap = 2;
  const groupW = series.length * colW + (series.length - 1) * gap;
  const slotW = Math.max(groupW + 32, Math.min(160, 800 / Math.max(1, categories.length)));
  const W = Math.max(800, slotW * categories.length);
  return (
    <Figure
      title={title}
      legend={<Legend items={series} />}
      table={
        <table>
          <thead><tr><th>Sprint</th>{series.map((s) => <th key={s.name}>{s.name} ({unit})</th>)}</tr></thead>
          <tbody>
            {categories.map((c, i) => (
              <tr key={c}><td>{c}</td>{series.map((s) => <td key={s.name}>{fmt(s.values[i] ?? 0)}</td>)}</tr>
            ))}
          </tbody>
        </table>
      }
    >
      <svg viewBox={`0 0 ${W} ${H}`} className="plot" role="img" aria-label={`${title}: ${series.map((s) => s.name).join(" and ")} per sprint`}>
        <line x1={0} x2={W} y1={base} y2={base} className="grid" />
        {categories.map((c, i) => {
          const off = (W - slotW * categories.length) / 2;
          const gx = off + i * slotW + (slotW - groupW) / 2;
          return (
            <g key={c}>
              {series.map((s, j) => {
                const v = s.values[i] ?? 0;
                const h = (v / max) * (base - top - 14);
                const x = gx + j * (colW + gap);
                return (
                  <g key={s.name}>
                    <title>{`${c} · ${s.name}: ${fmt(v)} ${unit}`}</title>
                    <path d={vbar(x, base, colW, Math.max(h, v > 0 ? 2 : 0))} fill={stroke(s.slot)} />
                    <text x={x + colW / 2} y={base - h - 4} textAnchor="middle" className="viz-value">{fmt(v)}</text>
                  </g>
                );
              })}
              <text x={off + i * slotW + slotW / 2} y={H - 6} textAnchor="middle" className="axis">{c}</text>
            </g>
          );
        })}
      </svg>
    </Figure>
  );
}

export function StatTile({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div className="stat">
      <span className="stat-label">{label}</span>
      <span className="stat-value">{value}</span>
      {hint && <span className="stat-hint muted">{hint}</span>}
    </div>
  );
}
