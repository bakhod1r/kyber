// chat: records a clip — a short video or voice message (S78) — with the browser's camera and microphone
// (infrastructure). Low bitrates keep five minutes of video under the 25 MB file limit.

export type ClipKind = "video" | "voice";
export type ClipRecorder = { stream: MediaStream; stop: () => Promise<Blob> };

export async function startClip(kind: ClipKind): Promise<ClipRecorder> {
  const stream = await navigator.mediaDevices.getUserMedia({ audio: true, video: kind === "video" ? { width: 640, height: 360 } : false });
  const types = kind === "video" ? ["video/webm;codecs=vp8,opus", "video/webm"] : ["audio/webm;codecs=opus", "audio/webm"];
  const mimeType = types.find((t) => MediaRecorder.isTypeSupported(t));
  const rec = new MediaRecorder(stream, { ...(mimeType ? { mimeType } : {}), videoBitsPerSecond: 450_000, audioBitsPerSecond: 32_000 });
  const chunks: Blob[] = [];
  rec.ondataavailable = (e) => e.data.size && chunks.push(e.data);
  rec.start(1000);
  return {
    stream,
    stop: () =>
      new Promise<Blob>((resolve) => {
        rec.onstop = () => {
          stream.getTracks().forEach((t) => t.stop());
          resolve(new Blob(chunks, { type: kind === "video" ? "video/webm" : "audio/webm" }));
        };
        rec.stop();
      }),
  };
}
