import { useCallback, useEffect, useMemo, useState } from 'react'
import { cache } from '@noverse/ui/data'
import { keys } from '../../../data/keys'
import { useResource } from '../../../data/useResource'
import {
  cancelTemplatePreview, getTemplatePreview, startTemplatePreview,
  type PreviewSource, type PreviewState, type PreviewTask, type TemplateDetail,
} from '../../../api/library'
import { t } from '../../../i18n/library'

/** Khoảng hỏi lại trạng thái khi việc dựng đang chạy — khớp nhịp thanh việc nền. */
const POLL_MS = 1500

export type TemplatePreviewControl = {
  /** `building` thắng mọi trạng thái; sửa chưa lưu làm `fresh` thành `stale`. */
  state: PreviewState
  builtAt: string | null
  projectId?: string
  jobId?: string
  minutes: number
  task?: PreviewTask
  sources: PreviewSource[]
  /** Lần dựng gần nhất lỗi — hiện ngay dưới nút. */
  error: string | null
  busy: boolean
  start: (jobId?: string, minutes?: number) => Promise<void>
  cancel: () => Promise<void>
}

/**
 * Trạng thái sống của bản xem thử: GET khi mở, hỏi lại mỗi 1,5 giây trong lúc
 * dựng, POST để dựng, DELETE để huỷ. Trạng thái trong `detail.preview` là bản
 * đầu tiên trước khi GET trả về.
 */
export function useTemplatePreview(templateId: string, detail: TemplateDetail, dirty: boolean): TemplatePreviewControl {
  const copy = t.library.template.preview
  const status = useResource(keys.templatePreview(templateId), ({ signal }) => getTemplatePreview(templateId, signal))
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const state: PreviewState = status.data?.state === 'building'
    ? 'building'
    : dirty && status.data?.state === 'fresh'
      ? 'stale'
      : status.data?.state ?? detail.preview.state
  const building = state === 'building'

  useEffect(() => {
    if (!building) return
    const timer = setInterval(() => { void status.refresh() }, POLL_MS)
    return () => clearInterval(timer)
  }, [building, status.refresh])

  const start = useCallback(async (jobId?: string, minutes?: number) => {
    setBusy(true)
    setError(null)
    try {
      const next = await startTemplatePreview(templateId, { jobId, minutes })
      await status.refresh()
      void cache.refresh(keys.templates())
      return void next
    } catch (err) {
      setError(err instanceof Error && err.message ? err.message : copy.startFailed)
    } finally {
      setBusy(false)
    }
  }, [templateId, status.refresh, copy.startFailed])

  const cancel = useCallback(async () => {
    setBusy(true)
    setError(null)
    try {
      await cancelTemplatePreview(templateId)
      await status.refresh()
    } catch (err) {
      setError(err instanceof Error && err.message ? err.message : copy.cancelFailed)
    } finally {
      setBusy(false)
    }
  }, [templateId, status.refresh, copy.cancelFailed])

  const sources = useMemo(() => status.data?.sources ?? [], [status.data])
  // Lỗi dựng gần nhất do backend giữ nguyên câu hành động (lỗi có mã + câu
  // phục hồi); dòng việc còn sót sau khởi động lại chỉ có `phase`, nên dùng
  // phase làm câu giải thích thay vì thay bằng câu chung.
  const failure = status.data?.error ?? null
  const failureText = failure ? [failure.message, failure.recovery].filter(Boolean).join(' · ') : null
  const taskError = status.data?.task?.state === 'failed' ? (status.data.task.phase || copy.buildFailed) : null
  return {
    state,
    builtAt: status.data?.builtAt ?? detail.preview.builtAt,
    projectId: status.data?.projectId ?? detail.preview.projectId,
    jobId: status.data?.jobId ?? detail.preview.jobId,
    minutes: status.data?.minutes ?? detail.preview.minutes ?? 3,
    task: status.data?.task,
    sources,
    error: error ?? failureText ?? taskError,
    busy,
    start,
    cancel,
  }
}
