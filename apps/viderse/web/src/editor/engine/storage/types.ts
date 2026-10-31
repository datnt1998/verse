/**
 * Wire shapes exchanged with `/api/editor/*`. They mirror the interfaces in
 * `src/server/editor/projects.ts` and `src/server/editor/media-probe.ts`;
 * the engine cannot import server modules, so the subset the client needs
 * is duplicated here.
 */
import type { MediaAttribution } from '@editor/types/storage'

export interface ServerProjectSummary {
  id: string
  name: string
  updatedAt: number
  createdAt?: number
}

export interface MediaProbeResult {
  kind: 'video' | 'audio' | 'image'
  mime: string
  durationSec: number | null
  width: number | null
  height: number | null
  fps: number | null
  videoCodec: string | null
  audioCodec: string | null
  sampleRate: number | null
  channels: number | null
}

/** Server-computed background classification for overlay media. */
export interface MediaBackgroundInfo {
  kind: 'black' | 'green' | 'none'
  keyColor?: string
  version: number
  detectedAt: number
  contentHash: string
}

export interface ServerMediaRecord {
  id: string
  fileName: string
  mime: string
  size: number
  /** `sha256:<hex>` — changes whenever the media bytes are replaced. */
  contentHash: string
  createdAt: number
  probe: MediaProbeResult
  tags?: string[]
  attribution?: MediaAttribution
  background?: MediaBackgroundInfo
  /** True when the media was uploaded as a reusable overlay layer. */
  overlay?: boolean
}

export interface ServerExportRecord {
  id: string
  fileName: string
  mime: string
  size: number
  status: 'staged' | 'ready'
  createdAt: number
  committedAt: number | null
}

/** Server-side error payload shape (`{ error, code, ...extra }`). */
export interface ServerErrorBody {
  error?: string
  code?: string
  revision?: string
}
