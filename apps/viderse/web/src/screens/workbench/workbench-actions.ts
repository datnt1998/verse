import type { CloneJobView } from '../../api/workbench'

export function editorProjectId(job: CloneJobView) {
  const value = job.steps?.find(step => step.step === 'build')?.output?.data?.projectId
  return typeof value === 'string' && value ? value : undefined
}

// Lượt làm lại vào hàng chờ trước; khi được nhận nó thành job mới cùng nguồn, thế hệ kế tiếp.
export function restartTarget(jobs: CloneJobView[], queueItemId: string, previous: CloneJobView) {
  return jobs.find(item => item.id === queueItemId)?.id
    ?? jobs.find(item => item.sourceKey === previous.sourceKey && item.generation === previous.generation + 1)?.id
}
