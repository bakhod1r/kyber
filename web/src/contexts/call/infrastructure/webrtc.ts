// call: the browser's WebRTC and media-device APIs, and the LiveKit media server (infrastructure).
import type { Platform, RoomConnection, RoomEvents } from "../domain/model";

export const browserPlatform: Platform = {
  peer: (config) => new RTCPeerConnection(config),
  userMedia: (c) => navigator.mediaDevices.getUserMedia(c),
  displayMedia: () => navigator.mediaDevices.getDisplayMedia({ video: true, audio: false }),
  devices: () => navigator.mediaDevices.enumerateDevices(),
  room: connectRoom,
};

// A room on the LiveKit media server (ADR-0017). livekit-client loads only when a room call is joined. Our own
// <video>/<audio> elements play the streams, so LiveKit's adaptive stream (which watches its own elements) stays
// off; dynacast still stops layers nobody watches.
type Kind = "camera" | "screen";

async function connectRoom(url: string, token: string, events: RoomEvents): Promise<RoomConnection> {
  const { Room, RoomEvent, Track } = await import("livekit-client");
  const room = new Room({ adaptiveStream: false, dynacast: true });
  const remote = new Map<string, Record<Kind, MediaStreamTrack[]>>();
  const kindOf = (source: string): Kind => (source === Track.Source.ScreenShare || source === Track.Source.ScreenShareAudio ? "screen" : "camera");
  const emit = (user: string, kind: Kind) => {
    const tracks = remote.get(user)?.[kind] ?? [];
    events.track(user, kind, tracks.length ? new MediaStream(tracks) : undefined);
  };
  const change = (user: string, source: string, track: MediaStreamTrack, add: boolean) => {
    const kind = kindOf(source);
    const mine = remote.get(user) ?? { camera: [], screen: [] };
    mine[kind] = add ? [...mine[kind].filter((t) => t !== track), track] : mine[kind].filter((t) => t !== track);
    remote.set(user, mine);
    emit(user, kind);
  };
  room.on(RoomEvent.TrackSubscribed, (track, pub, participant) => change(participant.identity, pub.source, track.mediaStreamTrack, true));
  room.on(RoomEvent.TrackUnsubscribed, (track, pub, participant) => change(participant.identity, pub.source, track.mediaStreamTrack, false));
  room.on(RoomEvent.ParticipantDisconnected, (participant) => {
    remote.delete(participant.identity);
    events.left(participant.identity);
  });
  await room.connect(url, token);
  return {
    publish: async (track, screen) => {
      const audio = track.kind === "audio";
      const source = screen ? (audio ? Track.Source.ScreenShareAudio : Track.Source.ScreenShare) : audio ? Track.Source.Microphone : Track.Source.Camera;
      await room.localParticipant.publishTrack(track, { source, simulcast: !audio });
    },
    unpublish: async (track) => {
      await room.localParticipant.unpublishTrack(track, false);
    },
    close: () => void room.disconnect(),
  };
}
