// @vitest-environment node

import { describe, expect, it } from 'vitest'
import { serverMediaToMetadata } from './media-adapter'
import type { ServerMediaRecord } from './types'

const videoRecord: ServerMediaRecord = {
  id: 'm1',
  fileName: 'clip.mp4',
  mime: 'video/mp4',
  size: 2048,
  contentHash: 'sha256:abc',
  createdAt: 1700,
  probe: {
    kind: 'video',
    mime: 'video/mp4',
    durationSec: 12.5,
    width: 1280,
    height: 720,
    fps: 29.97,
    videoCodec: 'h264',
    audioCodec: 'aac',
    sampleRate: 48000,
    channels: 2,
  },
}

describe('serverMediaToMetadata', () => {
  it('maps a video record onto the editor media shape', () => {
    expect(serverMediaToMetadata(videoRecord)).toEqual({
      id: 'm1',
      storageType: 'workspace',
      contentHash: 'sha256:abc',
      fileName: 'clip.mp4',
      fileSize: 2048,
      mimeType: 'video/mp4',
      duration: 12.5,
      width: 1280,
      height: 720,
      fps: 29.97,
      codec: 'h264',
      bitrate: 0,
      audioTrackPresent: true,
      audioCodec: 'aac',
      tags: [],
      attribution: undefined,
      createdAt: 1700,
      updatedAt: 1700,
    })
  })

  it('maps audio: codec is the audio codec, no frame rate or size', () => {
    const media = serverMediaToMetadata({
      ...videoRecord,
      mime: 'audio/mpeg',
      probe: {
        kind: 'audio',
        mime: 'audio/mpeg',
        durationSec: 3,
        width: null,
        height: null,
        fps: null,
        videoCodec: null,
        audioCodec: 'mp3',
        sampleRate: 44100,
        channels: 2,
      },
    })

    expect(media).toMatchObject({ codec: 'mp3', fps: 0, width: 0, height: 0, duration: 3, audioTrackPresent: true })
    expect(media.audioCodec).toBeUndefined()
  })

  it('marks a video probe with a null audio codec as a confirmed silent source', () => {
    // ffprobe saw every stream and reported none of type audio. This explicit
    // false is the only signal the mixer may use to skip a video decode.
    const media = serverMediaToMetadata({
      ...videoRecord,
      probe: { ...videoRecord.probe, audioCodec: null, sampleRate: null, channels: null },
    })

    expect(media.audioTrackPresent).toBe(false)
    expect(media.audioCodec).toBeUndefined()
  })

  it('maps images with zero duration and an unknown codec', () => {
    const media = serverMediaToMetadata({
      ...videoRecord,
      mime: 'image/png',
      probe: {
        kind: 'image',
        mime: 'image/png',
        durationSec: null,
        width: 800,
        height: 600,
        fps: null,
        videoCodec: null,
        audioCodec: null,
        sampleRate: null,
        channels: null,
      },
    })

    expect(media).toMatchObject({ duration: 0, width: 800, height: 600, fps: 0, codec: 'unknown' })
  })

  it('falls back to 30fps only when a video probe has no frame rate', () => {
    const media = serverMediaToMetadata({
      ...videoRecord,
      probe: { ...videoRecord.probe, fps: null },
    })

    expect(media.fps).toBe(30)
  })

  it('carries durable tags and attribution, copying the tag list', () => {
    const tags = ['voiceover']
    const attribution = { provider: 'LottieFiles', author: 'Ada' }

    const media = serverMediaToMetadata({ ...videoRecord, tags, attribution })

    expect(media.tags).toEqual(['voiceover'])
    expect(media.tags).not.toBe(tags)
    expect(media.attribution).toEqual(attribution)
  })
})
