import type { TimelineItem } from '@editor/types/timeline'
import { waveformReadiness } from './geometry'

/** Export/scoring can consume the same pure, hash-aware traversal. Only
 * referenced composition contents count; source audio is never synthesized. */
export function collectWaveformIssues(items: readonly TimelineItem[], compositions: readonly { id: string; items: TimelineItem[] }[] = [], media: readonly { id: string; contentHash?: string }[] = []): { itemId: string; message: string }[] {
  const issues: { itemId: string; message: string }[] = []
  const visited = new Set<string>()
  const walk = (items: readonly TimelineItem[]) => {
    for (const item of items) {
      if (item.type === 'shape' && item.waveformVisualizer) {
        const source = media.find(source => source.id === item.mediaId)
        const message = waveformReadiness(item, source?.contentHash)
        if (message) issues.push({ itemId: item.id, message })
      }
      if (item.type === 'composition' && !visited.has(item.compositionId)) {
        visited.add(item.compositionId)
        const composition = compositions.find(composition => composition.id === item.compositionId)
        if (composition) walk(composition.items)
      }
    }
  }
  walk(items)
  return issues
}
