// chat: application layer — the use cases the UI calls, and the live stream subscription.
import { useEffect, useRef } from "react";
import type { ChatEvent } from "../domain/model";
import { chatGateway, onConnect, openChatStream } from "../infrastructure/gateway";

export const chatUseCases = chatGateway;

/**
 * Subscribes to chat events while the component is mounted (one stream per tab); onConnect runs whenever the
 * stream (re)connects, to reload what may have been missed meanwhile.
 */
export function useChatStream(onEvent: (e: ChatEvent) => void, onConnected?: () => void) {
  const handler = useRef(onEvent);
  handler.current = onEvent;
  const connected = useRef(onConnected);
  connected.current = onConnected;
  useEffect(() => openChatStream((e) => handler.current(e)), []);
  useEffect(() => onConnect(() => connected.current?.()), []);
}

export * from "../domain/model";
export { startClip, type ClipKind, type ClipRecorder } from "../infrastructure/clip";
