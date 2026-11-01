import { assistantCopy as copy } from '../../i18n/assistant'

/** Gợi ý trong ô nhập theo trạng thái video (artboard Main › `d.placeholder`). */
export function workbenchPlaceholder(job: { stageGroup: string; activeStep: string | null }): string {
  const p = copy.composer.placeholders
  if (job.stageGroup === 'review') return p.review
  if (job.stageGroup !== 'working' && job.stageGroup !== 'attention' && job.stageGroup !== 'paused') return p.assistant
  switch (job.activeStep) {
    case 'download': case 'backdrop': case 'transcript': return p.scout
    case 'metadata': case 'scenes': return p.director
    case 'images': return job.stageGroup === 'paused' ? p.artistPaused : p.artist
    case 'build': case 'export': case 'score': return p.qa
    default: return p.assistant
  }
}
