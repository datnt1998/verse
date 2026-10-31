import type { CompositionInputProps } from '@editor/types/export'
import type { MediaMetadata } from '@editor/types/storage'
import type { SubComposition } from '@editor/features/timeline/stores/compositions-store'
import type {
  ClientExportSettings,
  ClientRenderResult,
  RenderProgress,
} from '../utils/client-renderer'

export interface ExportRenderStartRequest {
  type: 'start'
  requestId: string
  settings: ClientExportSettings
  composition: CompositionInputProps
  media: MediaMetadata[]
  compositions: SubComposition[]
  /** Source URLs are resolved in the host, retaining request-scoped authorization. */
  mediaUrls: Record<string, string>
}

export interface ExportRenderCancelRequest {
  type: 'cancel'
  requestId: string
}

export type ExportRenderWorkerRequest = ExportRenderStartRequest | ExportRenderCancelRequest

export interface ExportRenderProgressResponse {
  type: 'progress'
  requestId: string
  progress: RenderProgress
}

export interface ExportRenderCompleteResponse {
  type: 'complete'
  requestId: string
  result: ClientRenderResult
}

export interface ExportRenderCancelledResponse {
  type: 'cancelled'
  requestId: string
}

export interface ExportRenderErrorResponse {
  type: 'error'
  requestId: string
  error: string
}

export type ExportRenderWorkerResponse =
  | ExportRenderProgressResponse
  | ExportRenderCompleteResponse
  | ExportRenderCancelledResponse
  | ExportRenderErrorResponse
