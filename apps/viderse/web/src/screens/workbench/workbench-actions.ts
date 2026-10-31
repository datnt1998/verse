import { confirm } from '@noverse/ui'
import { jobBulkAction, type CloneJobView } from '../../api/workbench'

export function editorProjectId(job: CloneJobView) {
  const value = job.steps?.find(step => step.step === 'build')?.output?.data?.projectId
  return typeof value === 'string' && value ? value : undefined
}

// Lượt làm lại vào hàng chờ trước; khi được nhận nó thành job mới cùng nguồn, thế hệ kế tiếp.
export function restartTarget(jobs: CloneJobView[], queueItemId: string, previous: CloneJobView) {
  return jobs.find(item => item.id === queueItemId)?.id
    ?? jobs.find(item => item.sourceKey === previous.sourceKey && item.generation === previous.generation + 1)?.id
}

export async function markPosted(job: CloneJobView, onRefresh: () => void) {
  await jobBulkAction(job.id, 'posted')
  onRefresh()
  if (!job.exportBytes || job.exportBytes <= 0) return
  const mb = Math.ceil(job.exportBytes / (1024 * 1024))
  await confirm({
    title: `Xoá MP4 để lấy lại ${mb} MB?`,
    body: 'Video đã được đánh dấu là đã đăng. Xoá file MP4 trên máy để lấy lại dung lượng; ảnh và thông tin video vẫn được giữ.',
    cancelLabel: 'Để sau',
    confirmLabel: 'Xoá MP4',
    onConfirm: async () => { await jobBulkAction(job.id, 'deleteExport'); onRefresh() },
  })
}
