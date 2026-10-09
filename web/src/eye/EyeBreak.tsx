import { useEffect, useRef, useState } from "react";
import { LIBRARY, positionAt, stepAt, timeline } from "./exercises";

const TL = timeline(LIBRARY);
const R = 110; // how far the guide travels from the centre (SVG units)

/** A Sharingan-style iris: red iris, black pupil and three tomoe. */
function Iris({ closed }: { closed: boolean }) {
  return (
    <g>
      <circle r="34" fill="#c81e2b" stroke="#2a0507" strokeWidth="3" />
      <circle r="21" fill="none" stroke="#2a0507" strokeWidth="1.5" opacity="0.7" />
      <circle r="9" fill="#120203" />
      {[0, 120, 240].map((deg) => (
        <g key={deg} transform={`rotate(${deg}) translate(0 -21)`}>
          <circle r="4.5" fill="#120203" />
          <path d="M 0 -4.5 Q 9 -4 7 6" stroke="#120203" strokeWidth="2.5" fill="none" strokeLinecap="round" />
        </g>
      ))}
      {closed && <rect x="-40" y="-40" width="80" height="80" rx="40" fill="var(--surface)" opacity="0.92" />}
    </g>
  );
}

/** Full-screen guided eye break after a Pomodoro (KYB-S50). */
export function EyeBreak({ onClose }: { onClose: (how: "done" | "skipped") => void }) {
  const [t, setT] = useState(0);
  const started = useRef(Date.now());
  const close = useRef(onClose);
  close.current = onClose;
  const done = t >= TL.total;

  useEffect(() => {
    if (done) return;
    const id = setInterval(() => setT((Date.now() - started.current) / 1000), 100);
    return () => clearInterval(id);
  }, [done]);
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && close.current("skipped");
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const at = stepAt(TL, t);
  const pos = at ? positionAt(at.step.direction, at.progress) : { x: 0, y: 0 };
  const closed = at?.step.direction === "closed";
  const blinking = at?.step.direction === "blink" && Math.floor(t * 2) % 2 === 1;
  return (
    <div className="eye-break" role="dialog" aria-modal="true" aria-label="Eye break">
      {done ? (
        <div className="eye-done">
          <h2>Well done — your eyes thank you</h2>
          <button type="button" autoFocus onClick={() => onClose("done")}>
            Back to work
          </button>
        </div>
      ) : (
        <>
          <p className="eye-exercise">{at?.step.exercise}</p>
          <svg viewBox="-160 -160 320 320" className="eye-stage" aria-hidden="true">
            <rect x="-150" y="-150" width="300" height="300" rx="24" fill="none" stroke="currentColor" opacity="0.15" />
            <g data-testid="eye-guide" transform={`translate(${Math.round(pos.x * R)} ${Math.round(pos.y * R)})`} className="eye-guide">
              <Iris closed={closed || blinking} />
            </g>
          </svg>
          <p role="status" aria-live="polite" className="eye-say">
            {at?.step.instruction}
          </p>
          <progress max={TL.total} value={t} aria-label="Break progress" />
          <button type="button" className="ghost" onClick={() => onClose("skipped")}>
            Skip break
          </button>
        </>
      )}
    </div>
  );
}
