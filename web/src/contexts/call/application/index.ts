// call: application layer — the huddle of a conversation as a React hook over HuddleEngine.
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useMemo, useState, useSyncExternalStore } from "react";
import type { Call, FollowView, PresentKind } from "../domain/model";
import { callGateway, onCallEvents } from "../infrastructure/gateway";
import { browserPlatform } from "../infrastructure/webrtc";
import type { Platform } from "../domain/model";
import { HuddleEngine } from "./engine";

export { HuddleEngine, type HuddleState, type Remote } from "./engine";
export * from "../domain/model";

const liveKey = (conversationId: string) => ["call", "live", conversationId];

/** The conversation's huddle: the live call (kept current by the stream) and this tab's part in it. */
export function useHuddle(conversationId: string, user: string, platform: Platform = browserPlatform) {
  const qc = useQueryClient();
  const live = useQuery({ queryKey: liveKey(conversationId), queryFn: () => callGateway.live(conversationId) });
  const engine = useMemo(() => new HuddleEngine(user, callGateway, platform), [user, platform]);
  const state = useSyncExternalStore((fn) => engine.onChange(fn), () => engine.snapshot);
  const [view, setView] = useState<FollowView | null>(null);

  useEffect(
    () =>
      onCallEvents(
        (c: Call) => {
          if (c.conversation_id !== conversationId) return;
          qc.setQueryData(liveKey(conversationId), c.live ? c : null);
          engine.update(c);
        },
        (s) => void engine.onSignal(s),
        (v) => {
          if (v.call_id === engine.snapshot.call?.id) setView(v);
        },
      ),
    [conversationId, engine, qc],
  );
  useEffect(() => () => void engine.leave(), [engine]); // leaving the page leaves the huddle

  const call = state.joined ? state.call : (live.data ?? null);
  return {
    call,
    state,
    join: async () => {
      await engine.join(conversationId);
      qc.setQueryData(liveKey(conversationId), engine.snapshot.call);
    },
    leave: async () => {
      await engine.leave();
      await qc.invalidateQueries({ queryKey: liveKey(conversationId) });
    },
    toggleMic: () => engine.toggleMic(),
    toggleCamera: (deviceId?: string) => engine.toggleCamera(deviceId),
    useMicrophone: (deviceId: string) => engine.useMicrophone(deviceId),
    shareScreen: () => engine.shareScreen(),
    devices: () => engine.devices(),
    /** The presenter's latest view (for follow me). */
    view,
    present: async (kind: PresentKind, ref: string) => {
      const id = engine.snapshot.call?.id;
      if (id) engine.update(await callGateway.present(id, kind, ref));
    },
    stopPresenting: async () => {
      const id = engine.snapshot.call?.id;
      if (id) engine.update(await callGateway.stopPresenting(id));
    },
    follow: (x: number, y: number, zoom = 1) => {
      const id = engine.snapshot.call?.id;
      if (id) void callGateway.follow(id, { x, y, zoom }).catch(() => {});
    },
  };
}

/** Whose audio is above the speaking threshold right now (Web Audio analysers, sampled 5×/s). */
export function useSpeaking(streams: Record<string, MediaStream | undefined>): Set<string> {
  const [speaking, setSpeaking] = useState<Set<string>>(new Set());
  const entries = Object.entries(streams).filter((e): e is [string, MediaStream] => !!e[1]?.getAudioTracks().length);
  const key = entries.map(([id, s]) => `${id}:${s.id}:${s.getAudioTracks().length}`).join("|");
  useEffect(() => {
    if (typeof AudioContext === "undefined" || entries.length === 0) return;
    const ctx = new AudioContext();
    const meters = entries.map(([id, s]) => {
      const analyser = ctx.createAnalyser();
      analyser.fftSize = 512;
      ctx.createMediaStreamSource(s).connect(analyser);
      return { id, analyser, buf: new Uint8Array(analyser.fftSize) };
    });
    const timer = setInterval(() => {
      const now = new Set<string>();
      for (const m of meters) {
        m.analyser.getByteTimeDomainData(m.buf);
        let peak = 0;
        for (const v of m.buf) peak = Math.max(peak, Math.abs(v - 128));
        if (peak > 12) now.add(m.id);
      }
      setSpeaking((prev) => (prev.size === now.size && [...now].every((x) => prev.has(x)) ? prev : now));
    }, 200);
    return () => {
      clearInterval(timer);
      void ctx.close();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- key captures the streams' identity
  }, [key]);
  return speaking;
}
