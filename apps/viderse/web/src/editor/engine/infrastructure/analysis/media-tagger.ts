/**
 * Type-only stub for the upstream `media-tagger` module.
 * The captioning providers behind it are ML and were dropped from the engine;
 * consumers that only need the `MediaCaption` shape (workspace captions,
 * caption items) keep compiling against this declaration.
 */

export interface SceneCaptionData {
  caption?: string
  shotType?: string
  subjects?: string[]
  action?: string
  setting?: string
  lighting?: string
  timeOfDay?: string
  weather?: string
}

export interface MediaCaption {
  timeSec: number
  text: string
  sceneData?: SceneCaptionData
  /** Workspace-relative path to a captured JPEG thumbnail for this scene. */
  thumbRelPath?: string
  /** Dense sentence embedding of the caption's embed-text. */
  embedding?: number[]
  /** Dominant-color palette in CIELAB with pixel-coverage weights. */
  palette?: Array<{ l: number; a: number; b: number; weight: number }>
}
