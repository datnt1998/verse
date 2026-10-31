import type { CompositionInputProps } from '@editor/types/export'
import type { MediaMetadata } from '@editor/types/storage'
import type { SubComposition } from '@editor/features/timeline/stores/compositions-store'

export interface AudioMeasurementStartRequest {
  type: 'start'
  requestId: string
  composition: CompositionInputProps
  /**
   * Media records for the project's library — the worker hydrates its
   * resolver stores from these so URL resolution never leaves the request.
   */
  media: MediaMetadata[]
  /** Reachable sub-compositions for nested audio (clone projects: usually none). */
  compositions: SubComposition[]
}

export interface AudioMeasurementCancelRequest {
  type: 'cancel'
  requestId: string
}

export interface AudioMeasurementAckRequest {
  type: 'ack'
  requestId: string
  /** Last chunk seq the uploader durably sent — bounds in-flight PCM. */
  seq: number
}

export type AudioMeasurementWorkerRequest =
  | AudioMeasurementStartRequest
  | AudioMeasurementCancelRequest
  | AudioMeasurementAckRequest
/** One raw PCM chunk; `bytes` is transferred, never copied. */
export interface AudioMeasurementPcmResponse {
  type: 'pcm'
  requestId: string
  seq: number
  sampleRate: number
  channels: number
  bytes: ArrayBuffer
}

export interface AudioMeasurementDoneResponse {
  type: 'done'
  requestId: string
  chunks: number
  samples: number
  sampleRate: number
  channels: number
  durationSec: number
}

export interface AudioMeasurementCancelledResponse {
  type: 'cancelled'
  requestId: string
}

export interface AudioMeasurementErrorResponse {
  type: 'error'
  requestId: string
  error: string
}

export type AudioMeasurementWorkerResponse =
  | AudioMeasurementPcmResponse
  | AudioMeasurementDoneResponse
  | AudioMeasurementCancelledResponse
  | AudioMeasurementErrorResponse
