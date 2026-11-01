import * as stylex from '@stylexjs/stylex'
import type { CloneJobView } from '../../api/workbench'
import { routePath } from '../../router'
import { space as s, corners as r } from '../../styles/viderse.stylex'
import { requestMarkPosted } from '../../components/performance'
import { editorProjectId } from './workbench-actions'
import { removeVideo, restartVideo, stopVideo } from './jobActions'

export function OverflowMenu({ job, onClose, onRefresh, onError }: { job: CloneJobView; onClose: () => void; onRefresh: () => void; onError: (message: string) => void }) {
  const project = editorProjectId(job)
  const report = (error: unknown) => onError(error instanceof Error ? error.message : 'Không thực hiện được')
  const restart = () => { onClose(); restartVideo(job, { onRefresh, onError: report }) }
  const remove = () => { onClose(); removeVideo(job, { onRefresh, onError: report }) }
  const stop = () => { onClose(); stopVideo(job, { onRefresh, onError: report }) }
  return <div role="menu" aria-label="Thao tác video" {...stylex.props(styles.menu)} onClick={event => { if ((event.target as HTMLElement).closest('a')) onClose() }} onKeyDown={event => { if (event.key === 'Escape') { event.stopPropagation(); onClose() } }}>
    {project ? <a role="menuitem" {...stylex.props(styles.item)} href={`#/editor/${encodeURIComponent(project)}`}>Mở trong editor<small>Chỉnh tay bố cục, chữ, nhạc</small></a> : <button role="menuitem" disabled {...stylex.props(styles.item)}>Mở trong editor<small>Project chưa sẵn sàng</small></button>}
    {!job.supersededAt && <button role="menuitem" {...stylex.props(styles.item)} onClick={restart}>Làm lại từ đầu<small>Tạo lượt mới cho cùng video, tốn lại hạn mức ảnh</small></button>}
    {(job.stageGroup === 'working' || job.stageGroup === 'paused') && <button role="menuitem" {...stylex.props(styles.item)} onClick={stop}>Dừng hẳn</button>}
    {job.stageGroup === 'ready' && <><a role="menuitem" {...stylex.props(styles.item)} href={`/api/clone/download?ids=${encodeURIComponent(job.id)}`}>Tải gói đăng</a><button role="menuitem" {...stylex.props(styles.item)} onClick={() => { onClose(); void requestMarkPosted(job, onRefresh).catch(report) }}>Đánh dấu đã đăng</button></>}
    {/^https?:\/\//i.test(job.source.url) && <a role="menuitem" {...stylex.props(styles.item)} href={job.source.url} target="_blank" rel="noreferrer">Mở video nguồn</a>}
    {job.previousJobId && <a role="menuitem" {...stylex.props(styles.item)} href={`#${routePath({ name: 'workbench', channel: job.channelId, video: job.previousJobId })}`}>Lượt trước</a>}
    <a role="menuitem" {...stylex.props(styles.item)} href={`#/channels/${encodeURIComponent(job.channelId)}`}>Mở kênh</a>
    <button role="menuitem" {...stylex.props(styles.item, styles.danger)} onClick={remove}>Xoá video<small>Xoá MP4, ảnh và project của video này</small></button>
  </div>
}
const styles = stylex.create({
  menu: { position: 'absolute', zIndex: 30, right: 0, top: '100%', width: 280, padding: s['--vv-s-6'], border: '1px solid var(--line-2)', borderRadius: r['--vv-r-12'], backgroundColor: 'var(--color-surface-3)', boxShadow: 'var(--dialog-shadow)' },
  item: { display: 'flex', flexDirection: 'column', gap: s['--vv-s-2'], width: '100%', padding: s['--vv-s-8'], border: 0, borderRadius: r['--vv-r-8'], textAlign: 'left', textDecoration: 'none', backgroundColor: 'transparent', color: 'var(--color-fg)', fontSize: 13, cursor: 'pointer' },
  danger: { color: 'var(--color-err)' },
})
