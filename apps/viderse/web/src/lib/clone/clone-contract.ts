// Client-only subset of yt-factory 2c201e6's clone contract. Database schema,
// Node hashing and server handler contracts deliberately stay outside the web bundle.
import type { CloneStatusWire } from './clone-view'

export const AWAITING_REVIEW_CODE = 'awaiting_review'
export const CLONE_STEP_NAMES = ['download', 'backdrop', 'transcript', 'metadata', 'scenes', 'images', 'build', 'score', 'export'] as const
export type CloneStepName = (typeof CLONE_STEP_NAMES)[number]
export type CloneStepStatus = 'pending' | 'running' | 'waiting' | 'succeeded' | 'failed' | 'cancelled' | 'stale'
export type CloneStatusSnapshot = CloneStatusWire
