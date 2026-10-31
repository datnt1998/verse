/**
 * Adapter exports for export dependencies.
 * Timeline modules should import export rendering utilities from here.
 */

export { convertTimelineToComposition } from '@editor/features/export/utils/timeline-to-composition'
export type { ClientExportSettings, RenderProgress } from '@editor/features/export/utils/client-renderer'

export const importCanvasRenderOrchestrator = () =>
  import('@editor/features/export/utils/canvas-render-orchestrator')
