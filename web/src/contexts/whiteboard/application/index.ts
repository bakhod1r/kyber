// whiteboard: application layer — a board being drawn together: optimistic local changes, one request at a
// time to the server (each carrying the version it is based on), undo/redo of my own changes, and the
// others' changes and cursors from the stream.
import { useQuery } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState } from "react";
import { applyLocal, type Element, type Elements, inverse, type Op, reconcile } from "../domain/model";
import { onWhiteboardEvents, whiteboardGateway } from "../infrastructure/gateway";

export * from "../domain/model";
export const whiteboardUseCases = whiteboardGateway;

export type Cursor = { x: number; y: number; at: number };

export function useWhiteboard(id: string, me: string) {
  const board = useQuery({ queryKey: ["whiteboard", id], queryFn: () => whiteboardGateway.get(id) });
  const [els, setElsState] = useState<Elements>({});
  const current = useRef<Elements>({});
  /** Updates the elements outside of React's updater (it may run twice), keeping a ref in step. */
  const setEls = useCallback((next: Elements | ((cur: Elements) => Elements)) => {
    current.current = typeof next === "function" ? next(current.current) : next;
    setElsState(current.current);
  }, []);
  const [cursors, setCursors] = useState<Record<string, Cursor>>({});
  const [error, setError] = useState<string>();
  const confirmed = useRef<Elements>({}); // what the server has, for the versions our next ops are based on
  const queue = useRef<Promise<unknown>>(Promise.resolve());
  const undo = useRef<Op[]>([]);
  const redo = useRef<Op[]>([]);
  const [, bump] = useState(0);

  useEffect(() => {
    if (!board.data) return;
    const loaded: Elements = {};
    for (const e of board.data.elements ?? []) loaded[e.id] = e;
    confirmed.current = loaded;
    setEls(loaded);
  }, [board.data, setEls]);

  useEffect(
    () =>
      onWhiteboardEvents(
        (e) => {
          if (e.whiteboard_id !== id || e.by === me) return;
          const results = e.elements.map((element) => ({ element }));
          confirmed.current = reconcile(confirmed.current, results);
          setEls((cur) => reconcile(cur, results));
        },
        (c) => {
          if (c.whiteboard_id === id && c.user_id !== me) setCursors((cur) => ({ ...cur, [c.user_id]: { x: c.x, y: c.y, at: Date.now() } }));
        },
      ),
    [id, me, setEls],
  );

  /** Sends one op after the previous ones, based on the latest confirmed version of its element. */
  const send = useCallback(
    (op: Op) => {
      queue.current = queue.current.then(async () => {
        const base = confirmed.current[op.element.id]?.version ?? 0;
        const element: Element = { ...op.element, version: op.op === "add" ? 0 : base };
        try {
          const results = await whiteboardGateway.apply(id, [{ op: op.op, element }]);
          confirmed.current = reconcile(confirmed.current, results);
          setEls((cur) => reconcile(cur, results));
          const refused = results.find((r) => !r.applied);
          setError(refused ? `Someone changed this meanwhile (${refused.error})` : undefined);
        } catch (e) {
          setError(e instanceof Error ? e.message : String(e));
        }
      });
    },
    [id, setEls],
  );

  const change = useCallback(
    (op: Op) => {
      undo.current.push(inverse(op, current.current[op.element.id]));
      redo.current = [];
      setEls(applyLocal(current.current, op));
      send(op);
      bump((n) => n + 1);
    },
    [send, setEls],
  );

  const step = (from: { current: Op[] }, to: { current: Op[] }) => {
    const op = from.current.pop();
    if (!op) return;
    to.current.push(inverse(op, current.current[op.element.id]));
    setEls(applyLocal(current.current, op));
    send(op);
    bump((n) => n + 1);
  };

  const lastCursor = useRef(0);
  return {
    board: board.data,
    loading: board.isLoading,
    elements: els,
    cursors,
    error,
    change,
    undo: () => step(undo, redo),
    redo: () => step(redo, undo),
    canUndo: undo.current.length > 0,
    canRedo: redo.current.length > 0,
    /** Shares my pointer, at most 10 times a second. */
    moveCursor: (x: number, y: number) => {
      if (Date.now() - lastCursor.current < 100) return;
      lastCursor.current = Date.now();
      void whiteboardGateway.cursor(id, x, y).catch(() => {});
    },
  };
}
