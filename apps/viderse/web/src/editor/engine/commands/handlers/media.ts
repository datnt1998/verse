import { z } from 'zod'
import { useMediaLibraryStore } from '@editor/features/media-library/stores/media-library-store'
import { executeCommands } from '../executor'
import { registerCommand } from '../registry'
import type { CommandIssue, Inverse, ProjectStateView } from '../types'
import { refreshSourceItems } from '@editor/storage/media-source-refresh'

const id = z.string().min(1)
const number = z.number().finite()
const attribution = z.object({
  provider: z.string(), author: z.string().optional(), authorUrl: z.string().optional(),
  sourceUrl: z.string().optional(), sourceId: z.string().optional(), license: z.string().optional(),
}).strict()
export const mediaMetadataSchema = z.object({
  id, storageType: z.literal('workspace'), contentHash: z.string().optional(), fileLastModified: number.optional(),
  fileName: z.string(), fileSize: number.nonnegative(), mimeType: z.string(),
  duration: number, width: number, height: number, fps: number, codec: z.string(), bitrate: number,
  audioCodec: z.string().optional(), audioCodecSupported: z.boolean().optional(), videoCodecSupported: z.boolean().optional(),
  audioTrackPresent: z.boolean().optional(),
  previewAudioOpfsPath: z.string().optional(), previewAudioMimeType: z.string().optional(), previewAudioConformedAt: number.optional(),
  keyframeTimestamps: z.array(number).optional(), gopInterval: number.optional(), thumbnailId: z.string().optional(),
  tags: z.array(z.string()), attribution: attribution.optional(),
  aiCaptions: z.array(z.object({
    timeSec: number, text: z.string(),
    sceneData: z.object({
      caption: z.string().optional(), shotType: z.string().optional(), subjects: z.array(z.string()).optional(),
      action: z.string().optional(), setting: z.string().optional(), lighting: z.string().optional(),
      timeOfDay: z.string().optional(), weather: z.string().optional(),
    }).strict().optional(),
    thumbRelPath: z.string().optional(), embedding: z.array(number).optional(),
    palette: z.array(z.object({ l: number, a: number, b: number, weight: number }).strict()).optional(),
  }).strict()).optional(),
  createdAt: number, updatedAt: number,
  background: z.object({
    kind: z.enum(['black', 'green', 'none']), keyColor: z.string().optional(),
    version: number, detectedAt: number, contentHash: z.string(),
  }).strict().optional(),
  overlay: z.boolean().optional(),
}).strict()
export const mediaItemPatchSchema = z.object({
  mediaId: id.optional(), label: z.string().optional(),
  src: z.string().optional(), audioSrc: z.string().optional(), thumbnailUrl: z.string().optional(),
  waveformData: z.array(number).optional(),
  sourceWidth: number.optional(), sourceHeight: number.optional(),
  sourceFps: number.optional(), sourceDuration: number.optional(),
}).strict()
const mediaCacheFields = z.enum([
  'src', 'audioSrc', 'thumbnailUrl', 'waveformData',
  'reverseConformSrc', 'reverseConformPath', 'reverseConformKey',
  'reverseConformPreviewSrc', 'reverseConformPreviewPath', 'reverseConformPreviewKey',
  'reverseConformPreviewUsesProxy', 'reverseConformStatus',
  'reverseConformPreviewIsSourceLevel', 'reverseConformPreviewSourceDuration',
  'reverseConformPreviewFps', 'reverseConformLocalStart',
])
const inverse = (): Inverse => ({ kind: 'patch', patch: {} })
function validateMedia(state: ProjectStateView, params: { mediaId: string }): CommandIssue[] {
  return state.mediaItems?.some((media) => media.id === params.mediaId) ? [] : [{ message: `Không tìm thấy tệp ${params.mediaId}`, path: 'mediaId' }]
}
function nested(commands: Parameters<typeof executeCommands>[0]): void {
  const result = executeCommands(commands)
  if (!result.ok) throw new Error(result.issues.map((issue) => issue.message).join('; '))
}

export function registerMediaCommands(): void {
  registerCommand({
    type: 'media.add', history: 'none', dirty: true, params: z.object({ media: mediaMetadataSchema }).strict(),
    apply(ctx, params) {
      const records = ctx.draft.mediaItems ?? []
      ctx.draft.mediaItems = records.some((media) => media.id === params.media.id)
        ? records.map((media) => media.id === params.media.id ? params.media : media)
        : [params.media, ...records]
      return inverse()
    },
  })
  registerCommand({
    type: 'media.refreshSource', history: 'none', dirty: false,
    params: z.object({ media: mediaMetadataSchema, src: z.string().min(1) }).strict(),
    apply(ctx, params) {
      ctx.draft.mediaItems = (ctx.draft.mediaItems ?? []).map(media => media.id === params.media.id ? params.media : media)
      const source = { width: params.media.width, height: params.media.height, fps: params.media.fps || ctx.draft.fps, duration: params.media.duration }
      const refreshedItems = <T extends ProjectStateView['items'][number]>(items: T[]): T[] => items.map(item => {
        if (item.mediaId !== params.media.id) return item
        const refreshed = { ...item }
        refreshSourceItems([refreshed], params.media.id, params.src, params.media.contentHash, source)
        return refreshed
      })
      ctx.draft.items = refreshedItems(ctx.draft.items)
      ctx.draft.compositions = ctx.draft.compositions.map(composition => ({ ...composition, items: refreshedItems(composition.items) }))
      ctx.draft.stashStack = ctx.draft.stashStack?.map(stash => ({ ...stash, items: refreshedItems(stash.items) }))
      if (ctx.draft.mainHolder) ctx.draft.mainHolder = { ...ctx.draft.mainHolder, items: refreshedItems(ctx.draft.mainHolder.items) }
      return inverse()
    },
  })
  registerCommand({
    type: 'media.update', history: 'none', dirty: true,
    params: z.object({ mediaId: id, updates: mediaMetadataSchema.omit({ id: true }).partial() }).strict(), validate: (state, params) => validateMedia(state, params),
    apply(ctx, params) {
      ctx.draft.mediaItems = (ctx.draft.mediaItems ?? []).map((media) => media.id === params.mediaId ? { ...media, ...params.updates } : media)
      return inverse()
    },
  })
  registerCommand({
    type: 'media.setRecords', history: 'none', params: z.object({ mediaItems: z.array(mediaMetadataSchema), resetImporting: z.boolean().default(false) }).strict(),
    validate: (_state, params) => new Set(params.mediaItems.map((media) => media.id)).size === params.mediaItems.length ? [] : [{ message: 'Danh sách tệp có mã trùng nhau', path: 'mediaItems' }],
    apply(ctx, params) {
      if (params.resetImporting) useMediaLibraryStore.setState({ importingIds: [] })
      ctx.draft.mediaItems = params.mediaItems
      return inverse()
    },
  })
  registerCommand({
    type: 'media.removeRecords', history: 'none', dirty: true, params: z.object({ mediaIds: z.array(id) }).strict(),
    apply(ctx, params) {
      const ids = new Set(params.mediaIds)
      ctx.draft.mediaItems = (ctx.draft.mediaItems ?? []).filter((media) => !ids.has(media.id))
      return inverse()
    },
  })
  registerCommand({
    type: 'media.remove', params: z.object({ mediaIds: z.array(id) }).strict(),
    apply(ctx, params) {
      const mediaIds = new Set(params.mediaIds)
      const scopes = [ctx.draft.items, ...ctx.draft.compositions.map((composition) => composition.items), ...(ctx.draft.stashStack ?? []).map((stash) => stash.items), ...(ctx.draft.mainHolder ? [ctx.draft.mainHolder.items] : [])]
      const itemIds = [...new Set(scopes.flatMap((items) => items.filter((item) => item.mediaId && mediaIds.has(item.mediaId)).map((item) => item.id)))]
      if (itemIds.length) nested([{ type: 'item.removeProjectItems', params: { itemIds } }])
      ctx.draft.mediaItems = (ctx.draft.mediaItems ?? []).filter((media) => !mediaIds.has(media.id))
      return { inverse: inverse(), result: itemIds }
    },
  })
  registerCommand({
    type: 'media.relinkItems',
    params: z.object({ itemIds: z.array(id), updates: mediaItemPatchSchema, clearFields: z.array(mediaCacheFields).default([]) }).strict(),
    validate(state, params) {
      const scopes = [state.items, ...state.compositions.map((composition) => composition.items), ...(state.stashStack ?? []).map((stash) => stash.items), ...(state.mainHolder ? [state.mainHolder.items] : [])]
      const ids = new Set(scopes.flatMap((items) => items.map((item) => item.id)))
      return params.itemIds.filter((itemId) => !ids.has(itemId)).map((itemId) => ({ message: `Không tìm thấy clip ${itemId}`, path: 'itemIds' }))
    },
    apply(_ctx, params) {
      nested(params.itemIds.map((itemId) => ({ type: 'item.updateProjectItem', params: { itemId, updates: params.updates, clearFields: params.clearFields } })))
      return inverse()
    },
  })
}
