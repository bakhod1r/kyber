// A stand-in for the MQTT client behind shared/stream (S88) that tests drive.
import { act } from "@testing-library/react";
import { type StreamClient, USER_TOPIC } from "../stream";

export class FakeStreamClient implements StreamClient {
  connects = 0;
  topics: string[] = [];
  ended = false;
  private onMessage: ((topic: string, payload: Uint8Array) => void) | undefined;
  private onConnected: (() => void) | undefined;

  connector() {
    return () => {
      this.connects++;
      this.ended = false;
      return Promise.resolve<StreamClient>(this);
    };
  }
  subscribe(topic: string) {
    this.topics.push(topic);
  }
  on(event: "message" | "connect", listener: ((topic: string, payload: Uint8Array) => void) | (() => void)) {
    if (event === "message") this.onMessage = listener as (topic: string, payload: Uint8Array) => void;
    else this.onConnected = listener as () => void;
  }
  end() {
    this.ended = true;
  }
  connect() {
    this.onConnected?.();
  }
  message(topic: string, text: string) {
    this.onMessage?.(topic, new TextEncoder().encode(text));
  }
  /** emit delivers an event of the given type on the user's own topic. */
  emit(type: string, data: unknown) {
    this.message(USER_TOPIC, JSON.stringify({ type, data }));
  }
}

/** flush lets the pending connect promise settle. */
export const flush = () => act(() => new Promise<void>((r) => setTimeout(r, 0)));
