import * as stylex from '@stylexjs/stylex'
import { confirm } from '@noverse/ui'
import { getWorkbench, jobAction, previewRestart, restartJob, type CloneJobView } from '../../api/workbench'
import { confirmDeleteVideo } from '../../components'
import { navigate, routePath } from '../../router'
import { space as s, corners as r } from '../../styles/viderse.stylex'
import { editorProjectId, markPosted, restartTarget } from './workbench-actions'

export function OverflowMenu({ job, onClose, onRefresh, onError }: { job: CloneJobView; onClose: () => void; onRefresh: () => void; onError: (message: string) => void }) {
  const project = editorProjectId(job)
  const report = (error: unknown) => onError(error instanceof Error ? error.message : 'Không thực hiện được')
  const restart = () => {
    onClose()
    void previewRestart(job.id).then(preview => confirm({
      title: 'Làm lại từ đầu?',
      body: <><p>Tạo lượt mới cho cùng video nguồn và kênh, theo template tiếp theo trong luân phiên: <b>{preview.template.name}</b>.</p><p>Lượt cũ và ảnh của nó được giữ nguyên. {preview.imageEstimate === null ? 'Chưa ước lượng được số ảnh cho lượt mới.' : `Lượt mới dùng khoảng ${preview.imageEstimate} ảnh trong hạn mức.`}</p><p>Đưa lên đầu hàng chờ. {preview.etaStart ? `Dự kiến bắt đầu ${new Date(preview.etaStart).toLocaleString('vi-VN')}.` : 'Chưa có giờ bắt đầu dự kiến.'}</p></>,
      cancelLabel: 'Huỷ',
      confirmLabel: 'Làm lại từ đầu',
      onConfirm: async () => {
        const result = await restartJob(job.id)
        const list = await getWorkbench(job.channelId)
        onRefresh()
        navigate(routePath({ name: 'workbench', channel: job.channelId, video: restartTarget(list.jobs ?? [], result.queueItem.id, job) }))
      },
    })).catch(report)
  }
  const remove = () => {
    onClose()
    void confirmDeleteVideo({ name: job.source.title, deleting: ['MP4', 'Ảnh đã tạo', 'Project của video'], keeping: ['Video nguồn', 'Kênh'], running: job.stageGroup === 'working', onConfirm: async () => { await jobAction(job.id, 'delete'); onRefresh(); navigate(routePath({ name: 'workbench', channel: job.channelId })) } }).catch(report)
  }
  const stop = () => {
    onClose()
    void confirm({ title: 'Dừng hẳn video?', body: 'Các bước đang làm sẽ dừng. Kết quả đã có được giữ lại.', cancelLabel: 'Huỷ', confirmLabel: 'Dừng hẳn', onConfirm: async () => { await jobAction(job.id, 'cancel'); onRefresh() } }).catch(report)
  }
  return <div role="menu" aria-label="Thao tác video" {...stylex.props(styles.menu)} onClick={event => { if ((event.target as HTMLElement).closest('a')) onClose() }} onKeyDown={event => { if (event.key === 'Escape') { event.stopPropagation(); onClose() } }}>
    {project ? <a role="menuitem" {...stylex.props(styles.item)} href={`#/editor/${encodeURIComponent(project)}`}>Mở trong editor<small>Chỉnh tay bố cục, chữ, nhạc</small></a> : <button role="menuitem" disabled {...stylex.props(styles.item)}>Mở trong editor<small>Project chưa sẵn sàng</small></button>}
    {!job.supersededAt && <button role="menuitem" {...stylex.props(styles.item)} onClick={restart}>Làm lại từ đầu<small>Tạo lượt mới cho cùng video, tốn lại hạn mức ảnh</small></button>}
    {(job.stageGroup === 'working' || job.stageGroup === 'paused') && <button role="menuitem" {...stylex.props(styles.item)} onClick={stop}>Dừng hẳn</button>}
    {job.stageGroup === 'ready' && <><a role="menuitem" {...stylex.props(styles.item)} href={`/api/clone/download?ids=${encodeURIComponent(job.id)}`}>Tải gói đăng</a><button role="menuitem" {...stylex.props(styles.item)} onClick={() => { onClose(); void markPosted(job, onRefresh).catch(report) }}>Đánh dấu đã đăng</button></>}
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
