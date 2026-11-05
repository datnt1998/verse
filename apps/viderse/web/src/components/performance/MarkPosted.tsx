// Đánh dấu đã đăng: hộp hỏi link YouTube (không bắt buộc) → POST /api/clone/:id/posted
// → gợi ý xoá MP4 lấy lại dung lượng. Dùng chung cho Bàn dựng (⋯, LiveStudio) và
// bản điện thoại. Hộp là ConfirmDialog của kit: lỗi hiện ngay trong hộp.
import * as stylex from '@stylexjs/stylex'
import { Field, confirm, toast } from '@noverse/ui'
import { ApiError } from '../../api/request'
import { errorField, markPostedWithLink } from '../../api/performance'
import { jobBulkAction, type CloneJobView } from '../../api/workbench'
import { performanceCopy } from '../../i18n/performance'
import { isYoutubeLink } from './performance-logic'
import { performanceStyles as sx } from './performance.stylex'

const copy = performanceCopy.mark

function messageOf(error: unknown): string {
  if (error instanceof ApiError && error.status === 0) return copy.offline
  if (error instanceof ApiError && errorField(error) === 'youtubeUrl') return copy.badLink
  return error instanceof Error ? error.message : copy.failed
}

function LinkField({ onChange }: { onChange: (value: string) => void }) {
  return (
    <div {...stylex.props(sx.dialogField)}>
      <label htmlFor="mark-posted-link" {...stylex.props(sx.dialogLabel)}>{copy.label}</label>
      <Field id="mark-posted-link" type="url" inputMode="url" autoComplete="off" spellCheck={false} autoFocus placeholder={copy.placeholder} onChange={(event) => onChange(event.target.value)} />
    </div>
  )
}

/** Xoá riêng file MP4 của một video đã đăng (giữ ảnh, project và video). Dùng chung cho Bàn dựng và bản điện thoại. */
export async function requestDeleteExport(job: CloneJobView, onDone: () => void) {
  if (!job.exportBytes || job.exportBytes <= 0) return
  const mb = Math.ceil(job.exportBytes / (1024 * 1024))
  await confirm({
    title: `Xoá MP4 để lấy lại ${mb} MB?`,
    body: 'Video đã được đánh dấu là đã đăng. Xoá file MP4 trên máy để lấy lại dung lượng; ảnh và thông tin video vẫn được giữ.',
    cancelLabel: 'Để sau',
    confirmLabel: 'Xoá MP4',
    onConfirm: async () => { await jobBulkAction(job.id, 'deleteExport'); onDone() },
  })
}

/** Hỏi link, đánh dấu đã đăng, rồi gợi ý xoá MP4. `onDone` chạy sau mỗi thay đổi dữ liệu (làm mới danh sách). */
export async function requestMarkPosted(job: CloneJobView, onDone: () => void): Promise<void> {
  let link = ''
  const marked = await confirm({
    title: copy.title,
    body: <><span>{copy.body}</span><LinkField onChange={(value) => { link = value }} /></>,
    cancelLabel: copy.cancel,
    confirmLabel: copy.ok,
    pendingLabel: copy.pending,
    onConfirm: async () => {
      if (!isYoutubeLink(link)) throw new Error(copy.badLink)
      try { await markPostedWithLink(job.id, link) } catch (error) { throw new Error(messageOf(error)) }
      onDone()
    },
  })
  if (!marked) return
  toast.ok(copy.done)
  await requestDeleteExport(job, onDone)
}
