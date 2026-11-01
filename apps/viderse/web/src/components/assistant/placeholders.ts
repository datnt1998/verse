import { assistantCopy as copy } from '../../i18n/assistant'

/** Gợi ý trong ô nhập theo trạng thái video (artboard Main › `d.placeholder`). */
export function workbenchPlaceholder(job: { stageGroup: string; activeStep: string | null }): string {
  const p = copy.composer.placeholders
  switch (job.stageGroup) {
    case 'paused': return p.artistPaused
    case 'review': return p.review
    case 'ready': case 'posted': return p.assistant
    case 'working': case 'attention':
      return job.activeStep === 'render' || job.activeStep === 'export' || job.activeStep === 'score' ? p.qa : p.artist
    default: return p.artist
  }
}
