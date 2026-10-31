/** Painted caption coverage accumulated from isolated render surfaces, not estimated font metrics. */
export interface RenderedCaptionCoverage {
  version: 1
  output: { width: number; height: number; fps: number }
  renderedFrames: number
  entries: Array<{
    /** Composition wrapper item IDs from root to the item's immediate composition. */
    instancePath: string[]
    itemId: string
    /** Bounds already include the item's own transform, in immediate composition pixels. */
    space: { width: number; height: number }
    /** Null means observed but no painted pixels, never missing measurement. */
    bounds: { x: number; y: number; width: number; height: number } | null
  }>
}

export interface ExportCommitMetadata {
  captionCoverage?: {
    projectRevision: string
    coverage: RenderedCaptionCoverage
  }
}
