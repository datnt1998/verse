import type { MediaMetadata } from '@editor/types/storage'
import type { WaveformVisualizer } from '@editor/features/waveform-visualizer/types'

export const MEDIA_SOURCE_CHANNEL = 'ytf-editor-media-sources'
export const MEDIA_SOURCE_EVENT = 'ytf-editor-media-source-changed'
export interface MediaSourceChanged { workspaceId: string; mediaId: string; contentHash: string }
export const SOURCE_CACHE_FIELDS = ['thumbnailUrl', 'waveformData', 'reverseConformSrc', 'reverseConformPath', 'reverseConformKey', 'reverseConformPreviewSrc', 'reverseConformPreviewPath', 'reverseConformPreviewKey', 'reverseConformPreviewUsesProxy', 'reverseConformStatus', 'reverseConformPreviewIsSourceLevel', 'reverseConformPreviewSourceDuration', 'reverseConformPreviewFps', 'reverseConformLocalStart'] as const
export const SOURCE_MEMORY_CACHES = ['blob', 'bitmap', 'thumbnail', 'filmstrip', 'waveform', 'gif', 'proxy', 'decoded-audio'] as const
export function canonicalMediaSourceUrl(mediaId: string, hash: string): string {
  return `/api/editor/media/${encodeURIComponent(mediaId)}?v=${encodeURIComponent(hash)}`
}
export function sourceRefreshPlan(message: unknown, workspaceId: string, current: Pick<MediaMetadata, 'id' | 'contentHash'> | undefined) {
  if (!message || typeof message !== 'object') return null
  const event = message as Partial<MediaSourceChanged>
  if (event.workspaceId !== workspaceId || !current || event.mediaId !== current.id || typeof event.contentHash !== 'string' || !/^sha256:[a-f0-9]{64}$/.test(event.contentHash) || event.contentHash === current.contentHash) return null
  return { mediaId: current.id, oldUrl: current.contentHash ? canonicalMediaSourceUrl(current.id, current.contentHash) : null, newUrl: canonicalMediaSourceUrl(current.id, event.contentHash), caches: SOURCE_MEMORY_CACHES }
}
export interface SourceGeometry { width: number; height: number; fps: number; duration: number }
export function refreshSourceItems<T extends { mediaId?: string; type?: string; src?: string; audioSrc?: string; waveformVisualizer?: WaveformVisualizer }>(items: T[], mediaId: string, src: string, contentHash: string | undefined, source?: SourceGeometry): void {
  for (const item of items) {
    if (item.mediaId !== mediaId) continue
    if (item.type === 'shape') {
      if (!item.waveformVisualizer) continue
      if (item.waveformVisualizer.sourceHash !== contentHash) {
        const payload = { ...item.waveformVisualizer }
        delete payload.envelopeBase64
        delete payload.durationSeconds
        delete payload.normalization
        item.waveformVisualizer = payload
      }
      continue
    }
    for (const field of SOURCE_CACHE_FIELDS) Reflect.deleteProperty(item, field)
    item.src = src
    if (item.type === 'video' || item.type === 'audio') item.audioSrc = src
    if (source) {
      Reflect.set(item, 'sourceWidth', source.width)
      Reflect.set(item, 'sourceHeight', source.height)
      Reflect.set(item, 'sourceFps', source.fps)
      Reflect.set(item, 'sourceDuration', Math.round(source.duration * source.fps))
    }
  }
}
/** Visit only editorial collections; never walk large DSP/sample arrays. */
export function visitSourceItems(value: unknown, visit: (item: { mediaId: string; type?: string; src?: string; audioSrc?: string; waveformVisualizer?: WaveformVisualizer }) => void): void {
  if (!value || typeof value !== 'object') return
  if (Array.isArray(value)) { for (const child of value) visitSourceItems(child, visit); return }
  const record = value as Record<string, unknown>
  if (typeof record.mediaId === 'string' && (['video', 'audio', 'image', 'lottie'].includes(String(record.type)) || (record.type === 'shape' && record.waveformVisualizer))) visit(record as { mediaId: string; type?: string; src?: string; audioSrc?: string; waveformVisualizer?: WaveformVisualizer })
  for (const key of ['items', 'compositions', 'stashStack', 'mainHolder']) visitSourceItems(record[key], visit)
}
export function publishMediaSourceChanged(event: MediaSourceChanged): void {
  if (typeof window === 'undefined') return
  window.dispatchEvent(new CustomEvent(MEDIA_SOURCE_EVENT, { detail: event }))
  if (typeof BroadcastChannel !== 'undefined') {
    const channel = new BroadcastChannel(MEDIA_SOURCE_CHANNEL)
    channel.postMessage(event)
    channel.close()
  }
}
