import { z } from 'zod'
import { registerCommand } from '../registry'
import type { CommandIssue } from '../types'
import type { TemplateBackdrop, TimelineTrack } from '../../types/timeline'
import { templateSlotSchema, trackJsonSchema } from './item'
import { templateBackgroundAppearanceSchema, templateBackdropSchema, templateScenesSchema } from '../../features/editing-templates/template-schemas'
import { getPresetTrackHeight } from '../../features/timeline/utils/track-heights'

const updates = z.object({ name: z.string().optional(), height: z.number().positive().optional(), locked: z.boolean().optional(), visible: z.boolean().optional(), muted: z.boolean().optional(), solo: z.boolean().optional(), syncLock: z.boolean().optional(), parentTrackId: z.string().nullable().optional(), isGroup: z.boolean().optional(), isCollapsed: z.boolean().optional(), kind: z.enum(['video','audio']).optional(), order: z.number().optional(), templateSlot: templateSlotSchema.nullable().optional(), templateLoudnessLufs: z.number().min(-24).max(-9).nullable().optional(), templateScenes: templateScenesSchema.nullable().optional(), templateBackdrop: templateBackdropSchema.nullable().optional(), templateBackgroundAppearance: templateBackgroundAppearanceSchema.nullable().optional() }).strict()

const SLOT_LABEL: Record<string, string> = { scenes: 'Ảnh cảnh', captions: 'Phụ đề', 'source-audio': 'Âm thanh gốc', 'background-video': 'Nền video' }

/**
 * Slot rules, shared by every write path (add, update, verbatim set): scenes
 * and captions ride video tracks, source-audio an audio track, and one slot
 * belongs to at most one track. `excludeTrackId` is the track being updated.
 */
export function templateSlotIssues(tracks: readonly (Pick<TimelineTrack, 'id' | 'kind' | 'templateSlot'> & { name?: string })[], slot: NonNullable<TimelineTrack['templateSlot']>, kind: 'video' | 'audio' | undefined, excludeTrackId?: string, path = 'updates.templateSlot'): CommandIssue[] {
  const issues: CommandIssue[] = []
  if (slot === 'source-audio' ? kind !== 'audio' : kind === 'audio') {
    issues.push({ code: 'track.slotKind', message: slot === 'source-audio' ? 'Vai trò Âm thanh gốc chỉ gán cho track âm thanh' : `Vai trò ${SLOT_LABEL[slot]} chỉ gán cho track video`, path })
  }
  const holder = tracks.find((track) => track.templateSlot === slot && track.id !== excludeTrackId)
  if (holder) {
    issues.push({ code: 'track.slotTaken', message: `Vai trò ${SLOT_LABEL[slot]} đã thuộc track "${holder.name ?? holder.id}"`, path })
  }
  return issues
}

/** The scenes recipe is meaningful only while the track holds the `scenes` slot. */
function templateScenesIssues(slot: TimelineTrack['templateSlot'], path: string): CommandIssue[] {
  return slot === 'scenes' ? [] : [{ code: 'track.scenesOnly', message: 'Công thức ảnh chỉ đặt trên track vai trò Ảnh cảnh', path }]
}

/** Backdrop channels are meaningful only on the `background-video` slot track, one entry per channel. */
function templateBackdropIssues(slot: TimelineTrack['templateSlot'], backdrop: TemplateBackdrop, path: string): CommandIssue[] {
  const issues: CommandIssue[] = []
  if (slot !== 'background-video') issues.push({ code: 'track.backdropOnly', message: 'Kênh nền chỉ đặt trên track vai trò Nền video', path })
  const ids = backdrop.channels.map((channel) => channel.channelId)
  if (new Set(ids).size !== ids.length) issues.push({ code: 'track.backdropDuplicate', message: 'Một kênh nền chỉ được thêm một lần', path })
  return issues
}

export function registerTrackCommands(): void {
  registerCommand({ type: 'track.add', params: updates.extend({ id: z.string().optional(), beforeTrackId: z.string().nullable().optional() }).strict(),
    validate: (state, params) => [
      ...(params.templateSlot == null ? [] : templateSlotIssues(state.tracks, params.templateSlot, params.kind, params.id, 'templateSlot')),
      ...(params.templateScenes ? templateScenesIssues(params.templateSlot ?? undefined, 'templateScenes') : []),
      ...(params.templateBackdrop ? templateBackdropIssues(params.templateSlot ?? undefined, params.templateBackdrop, 'templateBackdrop') : []),
      ...(params.templateBackgroundAppearance && params.templateSlot !== 'background-video' ? [{ code: 'track.backgroundAppearanceOnly', message: 'Cài đặt nền chỉ đặt trên track vai trò Nền video', path: 'templateBackgroundAppearance' }] : []),
    ],
    apply({ draft, ids }, params) {
    const { id, beforeTrackId, parentTrackId, templateSlot, templateLoudnessLufs, templateScenes, templateBackdrop, templateBackgroundAppearance, ...fields } = params
    const track: TimelineTrack = { ...fields, id: id ?? ids.mint(), name: fields.name ?? 'Track mới', height: fields.height ?? getPresetTrackHeight(), locked: fields.locked ?? false, visible: fields.visible ?? true, muted: fields.muted ?? false, solo: fields.solo ?? false, order: fields.order ?? 0, items: [], ...(parentTrackId ? { parentTrackId } : {}), ...(templateSlot ? { templateSlot } : {}), ...(templateLoudnessLufs != null ? { templateLoudnessLufs } : {}), ...(templateScenes ? { templateScenes } : {}), ...(templateBackdrop ? { templateBackdrop } : {}), ...(templateBackgroundAppearance ? { templateBackgroundAppearance } : {}) }
    if (draft.tracks.some(existing => existing.id === track.id)) throw new Error(`Duplicate track: ${track.id}`)
    const index = beforeTrackId ? draft.tracks.findIndex(existing => existing.id === beforeTrackId) : 0
    const insertion = Math.max(0,index)
    const tracks = [...draft.tracks]
    tracks.splice(insertion,0,track)
    draft.tracks = tracks.map((existing,order) => ({...existing,order}))
    return { inverse: { kind: 'patch', patch: {} }, result: track }
  } })
  registerCommand({ type: 'track.remove', params: z.object({ ids: z.array(z.string()).min(1) }).strict(), apply({ draft }, params) {
    const removed = new Set(params.ids)
    let changed = true
    while (changed) { changed = false; for (const track of draft.tracks) if (track.parentTrackId && removed.has(track.parentTrackId) && !removed.has(track.id)) { removed.add(track.id); changed = true } }
    const itemIds = new Set(draft.items.filter(item => removed.has(item.trackId)).map(item => item.id))
    draft.items = draft.items.filter(item => !itemIds.has(item.id))
    draft.transitions = draft.transitions.filter(transition => !itemIds.has(transition.leftClipId) && !itemIds.has(transition.rightClipId))
    draft.keyframes = draft.keyframes.filter(entry => !itemIds.has(entry.itemId))
    draft.tracks = draft.tracks.filter(track => !removed.has(track.id))
    return { kind: 'patch', patch: {} }
  } })
  registerCommand({ type: 'track.update', params: z.object({ id: z.string(), updates }).strict(),
    validate: (state, params) => {
      const track = state.tracks.find((entry) => entry.id === params.id)
      if (!track) return [{ message: `Unknown track: ${params.id}` }]
      const slot = params.updates.templateSlot === null ? undefined : params.updates.templateSlot ?? track.templateSlot
      const kind = params.updates.kind ?? track.kind
      const issues = slot ? templateSlotIssues(state.tracks, slot, kind, params.id) : []
      const scenes = params.updates.templateScenes === null ? undefined
        : params.updates.templateScenes ?? (slot === 'scenes' ? track.templateScenes : undefined)
      issues.push(...(scenes ? templateScenesIssues(slot, 'updates.templateScenes') : []))
      const backdrop = params.updates.templateBackdrop === null ? undefined
        : params.updates.templateBackdrop ?? (slot === 'background-video' ? track.templateBackdrop : undefined)
      issues.push(...(backdrop ? templateBackdropIssues(slot, backdrop, 'updates.templateBackdrop') : []))
      const appearance = params.updates.templateBackgroundAppearance === null ? undefined
        : params.updates.templateBackgroundAppearance ?? (slot === 'background-video' ? track.templateBackgroundAppearance : undefined)
      if (appearance && slot !== 'background-video') issues.push({ code: 'track.backgroundAppearanceOnly', message: 'Cài đặt nền chỉ đặt trên track vai trò Nền video', path: 'updates.templateBackgroundAppearance' })
      return issues
    },
    mergeKey: params => `track.update:${params.id}`, apply({ draft }, params) {
    if (!draft.tracks.some(track => track.id === params.id)) throw new Error(`Unknown track: ${params.id}`)
    if (params.updates.parentTrackId) {
      let parent = draft.tracks.find(track => track.id === params.updates.parentTrackId)
      if (!parent?.isGroup) throw new Error('Parent track must be a group')
      while (parent) { if (parent.id === params.id) throw new Error('Circular track group'); parent = draft.tracks.find(track => track.id === parent?.parentTrackId) }
    }
    const slotUpdate = params.updates.templateSlot
    draft.tracks = draft.tracks.map(track => track.id === params.id ? { ...track, ...params.updates, templateSlot: slotUpdate === null ? undefined : slotUpdate ?? track.templateSlot, templateLoudnessLufs: params.updates.templateLoudnessLufs === null ? undefined : params.updates.templateLoudnessLufs ?? track.templateLoudnessLufs, templateScenes: params.updates.templateScenes === null || (slotUpdate !== undefined && slotUpdate !== 'scenes') ? undefined : params.updates.templateScenes ?? track.templateScenes, templateBackdrop: params.updates.templateBackdrop === null || (slotUpdate !== undefined && slotUpdate !== 'background-video') ? undefined : params.updates.templateBackdrop ?? track.templateBackdrop, templateBackgroundAppearance: params.updates.templateBackgroundAppearance === null || (slotUpdate !== undefined && slotUpdate !== 'background-video') ? undefined : params.updates.templateBackgroundAppearance ?? track.templateBackgroundAppearance, parentTrackId: params.updates.parentTrackId === null ? undefined : params.updates.parentTrackId ?? track.parentTrackId } : track)
    return { kind: 'patch', patch: {} }
  } })
  registerCommand({ type: 'track.reorder', params: z.object({ ids: z.array(z.string()) }).strict(), apply({ draft }, params) {
    if (new Set(params.ids).size !== draft.tracks.length || params.ids.length !== draft.tracks.length) throw new Error('Track order must contain every track once')
    draft.tracks = params.ids.map((id,order) => { const track = draft.tracks.find(track => track.id === id); if (!track) throw new Error(`Unknown track: ${id}`); return {...track,order} })
    return { kind: 'patch', patch: {} }
  } })
  registerCommand({ type: 'track.set', params: z.object({ tracks: z.array(trackJsonSchema) }).strict(),
    validate: (_state, params) => params.tracks.flatMap(track => [
      ...(track.templateSlot ? templateSlotIssues(params.tracks, track.templateSlot, track.kind, track.id, 'tracks.templateSlot') : []),
      ...(track.templateScenes ? templateScenesIssues(track.templateSlot, 'tracks.templateScenes') : []),
      ...(track.templateBackdrop ? templateBackdropIssues(track.templateSlot, track.templateBackdrop, 'tracks.templateBackdrop') : []),
    ]),
    apply({ draft }, params) {
    // Canonical verbatim write: never use items-store setTracks here, which re-derives
    // height/name from local view preferences and diverges live state from the command.
    draft.tracks = params.tracks.map(track => ({ ...track, items: [] })) as TimelineTrack[]
    return { kind: 'patch', patch: {} }
  } })
}
