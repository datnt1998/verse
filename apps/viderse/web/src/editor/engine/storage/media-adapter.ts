import type { MediaMetadata } from '@editor/types/storage'
import type { ServerMediaRecord } from './types'

/**
 * The server only knows what its own probe can read. Rendering extras that only the
 * browser can compute (thumbnails, keyframes, codec support) are layered on top by
 * the derived-metadata overlay, keyed by `contentHash`.
 */
const DEFAULT_VIDEO_FPS = 30

/**
 * Map a server media record onto the editor's `MediaMetadata`.
 *
 * Source bytes are always server-managed, so `storageType` is `'workspace'` and
 * `contentHash` carries the version that cache entries are keyed by. Probe fields the
 * server could not read become `0`/`'unknown'` exactly as the client probe reports them.
 */
export function serverMediaToMetadata(record: ServerMediaRecord): MediaMetadata {
  const { probe } = record
  const codec =
    probe.kind === 'video' ? probe.videoCodec : probe.kind === 'audio' ? probe.audioCodec : null

  return {
    id: record.id,
    storageType: 'workspace',
    contentHash: record.contentHash,
    fileName: record.fileName,
    fileSize: record.size,
    mimeType: record.mime,
    duration: probe.durationSec ?? 0,
    width: probe.width ?? 0,
    height: probe.height ?? 0,
    fps: probe.kind === 'video' ? (probe.fps ?? DEFAULT_VIDEO_FPS) : 0,
    codec: codec ?? 'unknown',
    bitrate: 0,
    // The server probe saw every stream: a video with a null audio codec is
    // confirmed silent, while an audio container always carries audio. Images
    // and Lottie leave this undefined because the mixer never reads audio there.
    audioTrackPresent:
      probe.kind === 'video'
        ? probe.audioCodec === null
          ? false
          : probe.audioCodec
            ? true
            : undefined
        : probe.kind === 'audio'
          ? true
          : undefined,
    audioCodec: probe.kind === 'video' ? (probe.audioCodec ?? undefined) : undefined,
    tags: record.tags ? [...record.tags] : [],
    attribution: record.attribution,
    background: record.background ? { ...record.background } : undefined,
    overlay: record.overlay === true ? true : undefined,
    createdAt: record.createdAt,
    updatedAt: record.createdAt,
  }
}
