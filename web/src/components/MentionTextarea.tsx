import { type KeyboardEvent, useId, useRef, useState } from "react";
import type { Member } from "../api";

type Props = {
  label: string;
  value: string;
  onChange: (value: string) => void;
  members: Member[];
  rows?: number;
  placeholder?: string;
};

const MAX = 6;

/** The "@query" right before the caret, if the caret is inside a mention being typed. */
function activeToken(text: string, caret: number): { start: number; query: string } | null {
  const m = /(^|\s)@(\S*)$/.exec(text.slice(0, caret));
  if (!m) return null;
  return { start: caret - (m[2] ?? "").length - 1, query: (m[2] ?? "").toLowerCase() };
}

function matches(m: Member, q: string) {
  return m.email.toLowerCase().startsWith(q) || m.name.toLowerCase().split(/\s+/).some((w) => w.startsWith(q));
}

/** A textarea that suggests project members after "@" (ARIA 1.2 combobox with a listbox popup). */
export function MentionTextarea({ label, value, onChange, members, rows = 3, placeholder }: Props) {
  const id = useId();
  const ref = useRef<HTMLTextAreaElement>(null);
  const [caret, setCaret] = useState(0);
  const [active, setActive] = useState(0);
  const [dismissedAt, setDismissedAt] = useState<number | null>(null);

  const token = activeToken(value, caret);
  const options = token && token.start !== dismissedAt ? members.filter((m) => matches(m, token.query)).slice(0, MAX) : [];
  const open = options.length > 0;
  const current = Math.min(active, options.length - 1);

  function sync(el: HTMLTextAreaElement) {
    setCaret(el.selectionStart ?? el.value.length);
  }

  function pick(m: Member) {
    if (!token) return;
    const insert = `@${m.email} `;
    const next = value.slice(0, token.start) + insert + value.slice(caret);
    const pos = token.start + insert.length;
    onChange(next);
    setCaret(pos);
    setActive(0);
    requestAnimationFrame(() => {
      ref.current?.focus();
      ref.current?.setSelectionRange(pos, pos);
    });
  }

  function onKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (!open) return;
    const chosen = options[current];
    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        setActive((current + 1) % options.length);
        break;
      case "ArrowUp":
        e.preventDefault();
        setActive((current - 1 + options.length) % options.length);
        break;
      case "Enter":
      case "Tab":
        if (chosen) {
          e.preventDefault();
          pick(chosen);
        }
        break;
      case "Escape":
        e.preventDefault();
        setDismissedAt(token?.start ?? null);
        break;
    }
  }

  const listId = `${id}-mentions`;
  const optionId = (i: number) => `${id}-mention-${i}`;
  return (
    <div className="mention">
      <label htmlFor={`${id}-input`}>{label}</label>
      <textarea
        id={`${id}-input`}
        ref={ref}
        rows={rows}
        placeholder={placeholder}
        value={value}
        role="combobox"
        aria-autocomplete="list"
        aria-expanded={open}
        aria-controls={listId}
        aria-activedescendant={open ? optionId(current) : undefined}
        onChange={(e) => {
          onChange(e.target.value);
          sync(e.target);
          setActive(0);
        }}
        onSelect={(e) => sync(e.currentTarget)}
        onKeyDown={onKeyDown}
        onBlur={() => setDismissedAt(token?.start ?? null)}
        onFocus={() => setDismissedAt(null)}
      />
      {open && (
        <ul id={listId} role="listbox" aria-label="Mention a member" className="mention-list">
          {options.map((m, i) => (
            <li
              key={m.user_id}
              id={optionId(i)}
              role="option"
              aria-selected={i === current}
              onMouseDown={(e) => {
                e.preventDefault(); // keep focus in the textarea
                pick(m);
              }}
            >
              <span className="mention-name">{m.name}</span>
              <span className="mention-email">{m.email}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
