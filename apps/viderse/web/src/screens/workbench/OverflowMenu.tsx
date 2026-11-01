import * as stylex from '@stylexjs/stylex'
import { ActionMenu, ListRow, Sheet } from '@noverse/ui'
import type { CloneJobView } from '../../api/workbench'
import { t } from '../../i18n/vi'
import { navigate, routePath } from '../../router'
import { space as s } from '../../styles/viderse.stylex'
import { requestMarkPosted } from '../../components/performance'
import { editorProjectId } from './workbench-actions'
import { removeVideo, restartVideo, stopVideo } from './jobActions'

type Entry = { key: string; label: string; hint?: string; href?: string; onClick?: () => void; disabled?: boolean; danger?: boolean }

export function OverflowMenu({ job, onClose, onRefresh, onError, mobile }: { job: CloneJobView; onClose?: () => void; onRefresh: () => void; onError: (message: string) => void; mobile?: boolean }) {
  const project = editorProjectId(job)
  const lockedReason = job.stageGroup === 'working' ? t.confirm.deleteVideo.locked : null
  const report = (error: unknown) => onError(error instanceof Error ? error.message : 'Không thực hiện được')
  const restart = () => { onClose?.(); restartVideo(job, { onRefresh, onError: report }) }
  const remove = () => { onClose?.(); removeVideo(job, { onRefresh, onError: report }) }
  const stop = () => { onClose?.(); stopVideo(job, { onRefresh, onError: report }) }
  const markPosted = () => { onClose?.(); void requestMarkPosted(job, onRefresh).catch(report) }

  const entries: Entry[] = []
  entries.push(project
    ? { key: 'editor', label: 'Mở trong editor', hint: 'Chỉnh tay bố cục, chữ, nhạc', href: `#/editor/${encodeURIComponent(project)}` }
    : { key: 'editor', label: 'Mở trong editor', hint: 'Project chưa sẵn sàng', disabled: true })
  if (!job.supersededAt) entries.push({ key: 'restart', label: 'Làm lại từ đầu', hint: 'Tạo lượt mới cho cùng video, tốn lại hạn mức ảnh', onClick: restart })
  if (job.stageGroup === 'working' || job.stageGroup === 'paused') entries.push({ key: 'stop', label: 'Dừng hẳn', onClick: stop })
  if (job.stageGroup === 'ready') {
    entries.push({ key: 'download', label: 'Tải gói đăng', href: `/api/clone/download?ids=${encodeURIComponent(job.id)}` })
    entries.push({ key: 'posted', label: 'Đánh dấu đã đăng', onClick: markPosted })
  }
  if (/^https?:\/\//i.test(job.source.url)) entries.push({ key: 'source', label: 'Mở video nguồn', href: job.source.url })
  if (job.previousJobId) entries.push({ key: 'previous', label: 'Lượt trước', href: `#${routePath({ name: 'workbench', channel: job.channelId, video: job.previousJobId })}` })
  entries.push({ key: 'channel', label: 'Mở kênh', href: `#/channels/${encodeURIComponent(job.channelId)}` })
  entries.push({ key: 'remove', label: 'Xoá video', hint: lockedReason ?? 'Xoá MP4, ảnh và project của video này', onClick: remove, disabled: Boolean(lockedReason), danger: !lockedReason })

  if (mobile) return <Sheet open onClose={() => onClose?.()} title="Thao tác video" size="md" height="auto" bodyStyle={styles.mobileBody}>
    <div>
      {entries.map((entry) => <ListRow key={entry.key} title={entry.label} hint={entry.hint} destructive={entry.danger} chevron={Boolean(entry.href)}
        href={entry.href} onClick={entry.disabled ? undefined : entry.onClick} />)}
    </div>
  </Sheet>

  return <ActionMenu label="Thêm thao tác cho video" alwaysVisible width={280} onClose={onClose} items={entries.map((entry) => ({
    id: entry.key,
    label: entry.label,
    description: entry.hint,
    danger: entry.danger,
    disabled: entry.key === 'remove' ? false : entry.disabled,
    lockedReason: entry.key === 'remove' ? lockedReason ?? undefined : undefined,
    onSelect: () => {
      if (entry.href?.startsWith('#')) navigate(entry.href.slice(1))
      else if (entry.key === 'source' && entry.href) window.open(entry.href, '_blank', 'noopener,noreferrer')
      else if (entry.href) location.assign(entry.href)
      else entry.onClick?.()
    },
  }))} />
}
const styles = stylex.create({
  mobileBody: { paddingInline: s['--vv-s-16'] },
})
