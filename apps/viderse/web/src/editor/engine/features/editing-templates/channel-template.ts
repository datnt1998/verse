import type { SerializedCommand } from '@editor/commands/types'
import type { TimelineItem, TimelineTrack } from '@editor/types/timeline'
import type { Transition } from '@editor/types/transition'
import { videoLoopBarrier } from '@editor/features/timeline/utils/source-calculations'

/**
 * Channel-template rules, pure and shared by the clone start gate, the build
 * step and the editor banner:
 * - slot roles live on tracks (`templateSlot`), anchors on items
 *   (`templateAnchor`) — the clone fill replaces slot content and re-times the
 *   rest;
 * - a template must be 1920×1080 at 30 fps with a `scenes` and a
 *   `source-audio` slot; `captions` is optional (absent = no captions);
 * - every referenced media id must exist and every `full`-anchored video must
 *   be able to loop.
 */

export const TEMPLATE_SLOT_LABELS: Record<NonNullable<TimelineTrack['templateSlot']>, string> = {
  scenes: 'Ảnh cảnh',
  captions: 'Phụ đề',
  'source-audio': 'Âm thanh gốc',
  'background-video': 'Nền video',
}

export const TEMPLATE_ANCHOR_LABELS: Record<'full' | 'start' | 'end', string> = {
  full: 'Phủ hết video',
  start: 'Giữ mốc đầu',
  end: 'Giữ mốc cuối',
}

/** The slice of project state the validator and the fill need — engine-agnostic. */
export interface TemplateProjectShape {
  metadata: { width: number; height: number; fps: number }
  tracks: readonly TimelineTrack[]
  items: readonly TimelineItem[]
  /** Authored scene transitions seed the fill's per-boundary style; absent on older snapshots. */
  transitions?: readonly Transition[]
}

export interface TemplateIssue {
  /** `error` refuses a clone start; `info` is a banner note. */
  severity: 'error' | 'info'
  message: string
  /** The offending track/item when the issue is clickable. */
  trackId?: string
  itemId?: string
}

/** True when a video clip is linked to an audio companion on the same project. */
function hasLinkedAudio(items: readonly TimelineItem[], item: TimelineItem): boolean {
  return !!item.linkedGroupId && items.some((other) => other.type === 'audio' && other.linkedGroupId === item.linkedGroupId)
}

/**
 * Validate a project as a channel template. `mediaIds` are the ids the
 * project's library actually resolves — a referenced-but-deleted media blocks
 * the start just like a missing slot.
 */
export function validateChannelTemplate(
  project: TemplateProjectShape,
  mediaIds: ReadonlySet<string>,
): TemplateIssue[] {
  const issues: TemplateIssue[] = []
  const { width, height, fps } = project.metadata
  if (width !== 1920 || height !== 1080 || fps !== 30) {
    issues.push({ severity: 'error', message: `Template phải là 1920×1080 · 30 fps (đang ${width}×${height} · ${fps} fps)` })
  }

  const seenSlots = new Map<NonNullable<TimelineTrack['templateSlot']>, TimelineTrack>()
  for (const track of project.tracks) {
    const slot = track.templateSlot
    if (!slot) continue
    const first = seenSlots.get(slot)
    if (first) {
      issues.push({ severity: 'error', message: `Vai trò ${TEMPLATE_SLOT_LABELS[slot]} gán trùm lên track "${track.name}" (đã có "${first.name}")`, trackId: track.id })
      continue
    }
    seenSlots.set(slot, track)
    if (slot === 'source-audio' ? track.kind !== 'audio' : track.kind === 'audio') {
      issues.push({ severity: 'error', message: `Vai trò ${TEMPLATE_SLOT_LABELS[slot]} không hợp loại track "${track.name}"`, trackId: track.id })
    }
  }

  const scenesTrack = seenSlots.get('scenes')
  if (!scenesTrack) {
    issues.push({ severity: 'error', message: 'Chưa có track vai trò Ảnh cảnh' })
  } else if (!project.items.some((item) => item.trackId === scenesTrack.id && item.type === 'image')) {
    issues.push({ severity: 'info', message: 'Track ảnh cảnh chưa có clip mẫu — ảnh sẽ phủ hết khung hình', trackId: scenesTrack.id })
  }
  const audioTrack = seenSlots.get('source-audio')
  if (!audioTrack) {
    issues.push({ severity: 'error', message: 'Chưa có track vai trò Âm thanh gốc' })
  } else if (!project.items.some((item) => item.trackId === audioTrack.id && item.type === 'audio')) {
    issues.push({ severity: 'info', message: 'Track âm thanh gốc chưa có clip mẫu — âm thanh sẽ phát ở mức gốc', trackId: audioTrack.id })
  }
  const backgroundTrack = seenSlots.get('background-video')
  if (backgroundTrack && !backgroundTrack.templateBackdrop && !project.items.some((item) => item.trackId === backgroundTrack.id && item.type === 'video')) {
    issues.push({ severity: 'info', message: 'Track nền video chưa có clip — nền sẽ để trống', trackId: backgroundTrack.id })
  }

  const missingMedia = new Set<string>()
  for (const item of project.items) {
    if (item.mediaId && !mediaIds.has(item.mediaId) && !missingMedia.has(item.mediaId)) {
      missingMedia.add(item.mediaId)
      issues.push({ severity: 'error', message: `Media của clip "${item.label}" không còn trong thư viện — thêm lại media hoặc sửa template`, trackId: item.trackId, itemId: item.id })
    }
    if (item.templateAnchor === 'full' && item.type === 'video' && videoLoopBarrier(item, hasLinkedAudio(project.items, item)) !== null) {
      issues.push({ severity: 'error', message: `Clip "${item.label}" neo toàn video nhưng không lặp được — tắt tiếng clip hoặc gán mốc khác`, trackId: item.trackId, itemId: item.id })
    }
  }
  return issues
}


/** Track names the old clone recipe produced → slot roles. */
const LEGACY_SLOT_TRACK_NAMES: Record<'scenes' | 'captions' | 'source-audio', string> = {
  scenes: 'Hình ảnh cảnh',
  captions: 'Phụ đề',
  'source-audio': 'Âm thanh gốc',
}

/**
 * `track.update` commands that assign slot roles to a legacy clone project —
 * one batch, one undo step. Empty when any role is already set or the named
 * tracks are not all present, so nothing half-assigns.
 */
export function suggestTemplateRoleCommands(tracks: readonly TimelineTrack[]): SerializedCommand[] {
  if (tracks.some((track) => track.templateSlot)) return []
  const commands: SerializedCommand[] = []
  for (const [slot, name] of Object.entries(LEGACY_SLOT_TRACK_NAMES) as [keyof typeof LEGACY_SLOT_TRACK_NAMES, string][]) {
    const track = tracks.find((entry) => entry.name.trim() === name)
    if (!track) return []
    commands.push({ type: 'track.update', params: { id: track.id, updates: { templateSlot: slot } } })
  }
  return commands
}
