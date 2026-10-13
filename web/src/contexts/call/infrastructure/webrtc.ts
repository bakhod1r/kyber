// call: the browser's WebRTC and media-device APIs (infrastructure).
import type { Platform } from "../domain/model";

export const browserPlatform: Platform = {
  peer: (config) => new RTCPeerConnection(config),
  userMedia: (c) => navigator.mediaDevices.getUserMedia(c),
  displayMedia: () => navigator.mediaDevices.getDisplayMedia({ video: true, audio: false }),
  devices: () => navigator.mediaDevices.enumerateDevices(),
};
