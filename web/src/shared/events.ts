// Browser-side domain events between bounded contexts (the frontend's published language).
// A context announces what happened; others react without importing each other.

const ISSUE_DONE = "kyber:issue-done";

/** Announce that an issue reached a Done status (Focus offers an eye break). */
export function announceIssueDone(issueKey: string) {
  window.dispatchEvent(new CustomEvent(ISSUE_DONE, { detail: issueKey }));
}

/** React to issues reaching Done; returns the unsubscribe function. */
export function onIssueDone(handler: (issueKey: string) => void): () => void {
  const listener = (e: Event) => handler((e as CustomEvent<string>).detail);
  window.addEventListener(ISSUE_DONE, listener);
  return () => window.removeEventListener(ISSUE_DONE, listener);
}
