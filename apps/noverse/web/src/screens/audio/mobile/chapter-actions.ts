// Mở menu chỉ mô tả thao tác của chương đang chọn, không gọi máy chủ.
// Bản giọng hiện có được giữ nguyên cho tới khi người dùng xác nhận tạo lại.
import type { AudioSettings } from '../../../api/types'
import type { IconName } from '@noverse/ui'
import { t } from '../../../i18n/vi'
import type { GenRow } from '../genWork'
import { formatMinutes } from './logic'

export type ChapterActionId =
  | 'play'
  | 'pausePlayback'
  | 'export'
  | 'share'
  | 'update'
  | 'create'
  | 'redo'
  | 'retry'
  | 'prioritize'
  | 'pauseJob'
  | 'resumeJob'
  | 'cancelJob'

export type ChapterActionConfirm = {
  title: string
  body: string
  label: string
  destructive?: boolean
}

export type ChapterActionItem = {
  id: ChapterActionId
  label: string
  /** Dòng phụ 12.5px dưới nhãn. */
  detail: string
  icon: IconName
  tone?: 'warn' | 'danger'
  disabled?: boolean
  /** Việc không hoàn tác tức thì (tạo lại / huỷ) hỏi lại trước khi gọi máy chủ. */
  confirm?: ChapterActionConfirm
}

export type ChapterActionContext = {
  /** Chương đã đủ giọng để nghe/xuất (cùng `playerQueue`). */
  ready: boolean
  /** Đang phát đúng chương này. */
  playing: boolean
  writable: boolean
  quality: AudioSettings['quality']
}

/**
 * Danh sách mục cho menu ⋯: nghe · xuất riêng · chia sẻ · việc của trạng thái ·
 * tạo lại · huỷ. `hold`/`composing` không có thao tác (hàng đã tự
 * nói trạng thái) nên trả rỗng — trang Nghe ẩn nút ⋯ ở hai trạng thái đó.
 */
export function chapterActions(row: GenRow | null, ctx: ChapterActionContext): ChapterActionItem[] {
  if (!row) return []
  const c = t.audio.generation
  const a = t.audio.chapterActions
  const { chapter, job, state } = row
  const n = chapter.chapter
  const canWrite = ctx.writable
  const quality = c[ctx.quality]
  const locked = a.needVoiceWrite
  const noWrite = a.needWrite
  const items: ChapterActionItem[] = []

  if (ctx.ready) {
    items.push(ctx.playing
      ? { id: 'pausePlayback', label: c.pause, detail: a.playing, icon: 'pause' }
      : { id: 'play', label: a.play, detail: a.openPlayer, icon: 'play' })
    const seconds = chapter.audio?.durationSec ?? chapter.durationSec ?? 0
    items.push({
      id: 'export',
      label: a.export,
      detail: !canWrite ? a.needExportWrite : seconds > 0 ? a.fileDuration(formatMinutes(seconds)) : a.file,
      icon: 'export-square',
      disabled: !canWrite,
    })
    items.push({
      id: 'share',
      label: a.share,
      detail: a.shareDetail,
      icon: 'link-2',
    })
  }

  if (state === 'stale') {
    const segs = chapter.changedSegments > 0 ? `${c.changedSegs(chapter.changedSegments)} · ` : ''
    items.push({
      id: 'update',
      label: a.update,
      detail: canWrite ? `${segs}${chapter.chapterChanged ? c.chapterChangedDesc : c.scriptChangedDesc} · ${quality}` : locked,
      icon: 'refresh-2',
      tone: 'warn',
      disabled: !canWrite,
    })
  } else if (state === 'new') {
    const what = chapter.status === 'todo' ? c.composeFirst : `${chapter.segments} ${c.segments}`
    const issues = chapter.issues > 0 ? `${chapter.issues} ${c.review} · ` : ''
    const lockNote = chapter.status === 'locked' ? '' : ` · ${c.lockWarning}`
    items.push({
      id: 'create',
      label: a.create,
      detail: canWrite ? `${issues}${what} · ${quality}${lockNote}` : locked,
      icon: 'microphone-2',
      disabled: !canWrite,
    })
  } else if (state === 'err') {
    items.push({
      id: 'retry',
      label: c.retry,
      detail: !canWrite ? noWrite : job?.error || c.failed,
      icon: 'refresh-2',
      tone: 'danger',
      disabled: !canWrite || !job,
    })
  } else if (state === 'queue') {
    if (row.position > 1) items.push({
      id: 'prioritize',
      label: c.prioritize,
      detail: !canWrite ? noWrite : c.waitingAt(row.position),
      icon: 'arrow-up-2',
      disabled: !canWrite || !job,
    })
    items.push({
      id: 'pauseJob',
      label: a.pause,
      detail: !canWrite ? noWrite : c.waitingAt(row.position || 1),
      icon: 'pause',
      disabled: !canWrite || !job,
    })
  } else if (state === 'run') {
    items.push({
      id: 'pauseJob',
      label: a.pause,
      detail: !canWrite ? noWrite : job ? `${c.segment} ${job.segmentsDone}/${job.segments}` : c.active,
      icon: 'pause',
      disabled: !canWrite || !job,
    })
  } else if (state === 'paused') {
    items.push({
      id: 'resumeJob',
      label: a.resume,
      detail: !canWrite ? noWrite : c.kept,
      icon: 'play',
      disabled: !canWrite || !job,
    })
  }

  if (state === 'done' || state === 'stale') {
    items.push({
      id: 'redo',
      label: a.redo,
      detail: canWrite ? a.redoDetail(quality) : locked,
      icon: 'refresh-2',
      disabled: !canWrite,
      confirm: {
        title: a.redoTitle(n),
        body: a.redoBody,
        label: a.redoConfirm(n),
      },
    })
  }

  if (state === 'queue' || state === 'run' || state === 'paused') {
    items.push({
      id: 'cancelJob',
      label: a.cancel,
      detail: canWrite ? a.kept : noWrite,
      icon: 'close-circle',
      tone: 'danger',
      disabled: !canWrite || !job,
      confirm: {
        title: a.cancelTitle(n),
        body: a.cancelBody,
        label: a.cancel,
        destructive: true,
      },
    })
  }

  return items
}
