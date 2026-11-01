// Ba thao tác có hộp xác nhận của một video, dùng chung cho menu ⋯ của Bàn dựng
// và của bảng Kênh › Video: Làm lại từ đầu · Dừng hẳn · Xoá video.
import { confirm } from '@noverse/ui'
import { getWorkbench, jobAction, previewRestart, restartJob, type CloneJobView } from '../../api/workbench'
import { confirmDeleteVideo } from '../../components'
import { navigate, routePath } from '../../router'
import { restartTarget } from './workbench-actions'

type Callbacks = { onRefresh: () => void; onError: (error: unknown) => void }

export function restartVideo(job: CloneJobView, { onRefresh, onError }: Callbacks) {
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
  })).catch(onError)
}

/** `afterRemove` mặc định: về Bàn dựng của kênh (nơi gọi từ Bàn dựng); bảng Kênh giữ nguyên trang. */
export function removeVideo(job: CloneJobView, { onRefresh, onError, afterRemove }: Callbacks & { afterRemove?: () => void }) {
  void confirmDeleteVideo({
    name: job.source.title,
    deleting: ['MP4', 'Ảnh đã tạo', 'Project của video'],
    keeping: ['Video nguồn', 'Kênh'],
    running: job.stageGroup === 'working',
    onConfirm: async () => {
      await jobAction(job.id, 'delete')
      onRefresh()
      if (afterRemove) afterRemove()
      else navigate(routePath({ name: 'workbench', channel: job.channelId }))
    },
  }).catch(onError)
}

export function stopVideo(job: CloneJobView, { onRefresh, onError }: Callbacks) {
  void confirm({
    title: 'Dừng hẳn video?',
    body: 'Các bước đang làm sẽ dừng. Kết quả đã có được giữ lại.',
    cancelLabel: 'Huỷ',
    confirmLabel: 'Dừng hẳn',
    onConfirm: async () => { await jobAction(job.id, 'cancel'); onRefresh() },
  }).catch(onError)
}
