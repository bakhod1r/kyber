// chat: application layer — the use cases the UI calls, and the live stream subscription.
import { useEffect, useRef } from "react";
import type { ChatEvent } from "../domain/model";
import { chatGateway, openChatStream } from "../infrastructure/gateway";

export const chatUseCases = chatGateway;

/** Subscribes to chat events while the component is mounted (one stream per tab). */
export function useChatStream(onEvent: (e: ChatEvent) => void) {
  const handler = useRef(onEvent);
  handler.current = onEvent;
  useEffect(() => openChatStream((e) => handler.current(e)), []);
}

export * from "../domain/model";
