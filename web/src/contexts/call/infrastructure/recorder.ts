// call: records a huddle in the browser (S62) — every video drawn into one canvas grid, every audio track
// mixed, encoded by MediaRecorder as WebM (infrastructure).

export type Recorder = { stop: () => Promise<Blob> };

/** Starts recording; sources() is asked each frame, so people joining or leaving show up in the grid. */
export function startRecorder(sources: () => MediaStream[]): Recorder {
  const canvas = document.createElement("canvas");
  canvas.width = 1280;
  canvas.height = 720;
  const g = canvas.getContext("2d");
  const audio = new AudioContext();
  const mix = audio.createMediaStreamDestination();
  const wired = new Map<string, MediaStreamAudioSourceNode>();
  const videos = new Map<string, HTMLVideoElement>();

  const frame = () => {
    const streams = sources();
    for (const s of streams) {
      for (const t of s.getAudioTracks()) {
        if (!wired.has(t.id)) {
          const node = audio.createMediaStreamSource(new MediaStream([t]));
          node.connect(mix);
          wired.set(t.id, node);
        }
      }
    }
    const tracks = streams.flatMap((s) => s.getVideoTracks()).filter((t) => t.readyState === "live");
    if (!g) return;
    g.fillStyle = "#111";
    g.fillRect(0, 0, canvas.width, canvas.height);
    const cols = Math.ceil(Math.sqrt(tracks.length || 1));
    const rows = Math.ceil((tracks.length || 1) / cols);
    const w = canvas.width / cols, h = canvas.height / rows;
    tracks.forEach((t, i) => {
      let v = videos.get(t.id);
      if (!v) {
        v = document.createElement("video");
        v.muted = true;
        v.srcObject = new MediaStream([t]);
        void v.play().catch(() => {});
        videos.set(t.id, v);
      }
      if (v.videoWidth) g.drawImage(v, (i % cols) * w, Math.floor(i / cols) * h, w, h);
    });
  };
  const timer = window.setInterval(frame, 1000 / 15);
  frame();

  const out = new MediaStream([...canvas.captureStream(15).getVideoTracks(), ...mix.stream.getAudioTracks()]);
  const type = ["video/webm;codecs=vp9,opus", "video/webm;codecs=vp8,opus", "video/webm"].find((t) => MediaRecorder.isTypeSupported(t));
  const rec = new MediaRecorder(out, type ? { mimeType: type } : undefined);
  const chunks: Blob[] = [];
  rec.ondataavailable = (e) => e.data.size && chunks.push(e.data);
  rec.start(1000);

  return {
    stop: () =>
      new Promise<Blob>((resolve) => {
        rec.onstop = () => {
          window.clearInterval(timer);
          out.getTracks().forEach((t) => t.stop());
          videos.forEach((v) => (v.srcObject = null));
          void audio.close();
          resolve(new Blob(chunks, { type: "video/webm" }));
        };
        rec.stop();
      }),
  };
}
