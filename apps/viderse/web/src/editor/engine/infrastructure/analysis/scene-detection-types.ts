/**
 * Type-only stub for the upstream `scene-detection-types` module.
 * Scene detection is ML and was dropped; workspace storage modules still
 * persist payloads shaped like these types.
 */

export const SCENE_DETECTOR_VERSION = 2

export type SceneDetectionMethod = 'histogram' | 'adaptive'

export interface AdaptiveSceneCutMetrics {
  kind: 'adaptive'
  contentScore: number
  adaptiveRatio: number
  hueDelta: number
  saturationDelta: number
  lumaDelta: number
  returnScore?: number
}

export interface HistogramSceneCutMetrics {
  kind: 'histogram'
  histogramDistance: number
}

export type SceneCutMetrics = AdaptiveSceneCutMetrics | HistogramSceneCutMetrics

export interface SceneCut {
  /** Authoritative presentation time in the source media, in seconds. */
  time: number
  /** Raw detector score used to rank nearby candidates. */
  score: number
  /** Normalized detector confidence in the range 0..1. */
  confidence: number
  type: 'cut'
  metrics: SceneCutMetrics
  /** Whether a verification model confirmed the candidate. */
  verified?: boolean
}
