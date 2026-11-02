// Client "Vẽ thử ảnh" — khớp internal/web/template_trial.go.
//   GET  /api/templates/:id/cover-trials                          → CoverTrialStatus
//   POST /api/templates/:id/cover-trials { jobId, confirm }      → 202 CoverTrialStatus
//   POST /api/templates/:id/cover-trials/:trial/cancel           → 204
//   GET  /api/templates/:id/cover-trials/:trial/image            → ảnh PNG
//   GET  /api/image-prompts/:id/samples                          → PromptSampleStatus
//   POST /api/image-prompts/:id/samples { videoId, sceneId?, confirm } → 202 PromptSampleStatus
//   GET  /api/image-prompts/:id/samples/scenes?videoId=          → { items: TrialScene[] }
//   POST /api/image-prompts/:id/samples/:sample/cancel           → 204
//   GET  /api/image-prompts/:id/samples/:sample/image            → ảnh PNG
// Mỗi lần vẽ là MỘT ảnh thật tốn một lượt hạn mức; confirm:true là xác nhận
// của người dùng, máy chủ từ chối khi thiếu (409 confirm_required).
import { mutate, request } from './request'

export type TrialState = 'running' | 'ready' | 'failed' | 'cancelled'

export type TrialError = { code: string; message: string; recovery?: string; retryAt?: string }

export type TrialQuota = {
  provider: string
  label: string
  paid: boolean
  left: number
  total: number
  resetAt?: string
}

/** Giá của đúng một lần vẽ. quotaLeft = -1 khi không tính phí/chưa đọc được. */
export type TrialCost = { images: number; provider: string; label: string; quotaLeft: number }

export type TrialSource = { jobId: string; title: string; channelName?: string; durationSec: number }
export type TrialTask = { id: string; phase?: string; state: string; done: number; total: number; actionHref?: string }
export type TrialPrompt = { id: string; name: string; composition?: string }
export type TrialScene = { id: string; label: string }

export type CoverTrial = {
  id: string
  jobId: string
  jobTitle: string
  promptId: string
  promptName: string
  composition: string
  state: TrialState
  provider?: string
  model?: string
  width?: number
  height?: number
  costImages: number
  hookText?: string
  createdAt: string
  finishedAt?: string
  error?: TrialError
  imageUrl?: string
}

export type CoverTrialStatus = {
  sources: TrialSource[]
  history: CoverTrial[]
  task?: TrialTask
  quota?: TrialQuota
  cost: TrialCost
  prompt?: TrialPrompt
  ready: boolean
  blockedReason?: string
}

export type PromptSample = {
  id: string
  promptId: string
  promptName: string
  promptKind: 'scene' | 'cover'
  composition?: string
  videoId: string
  videoTitle: string
  sceneId?: string
  hookText?: string
  state: TrialState
  provider?: string
  model?: string
  width?: number
  height?: number
  costImages: number
  createdAt: string
  finishedAt?: string
  error?: TrialError
  imageUrl?: string
}

export type PromptSampleStatus = {
  sources: TrialSource[]
  history: PromptSample[]
  task?: TrialTask
  quota?: TrialQuota
  cost: TrialCost
  prompt?: TrialPrompt
  ready: boolean
  blockedReason?: string
}

export const getCoverTrials = (templateId: string, signal?: AbortSignal) =>
  request<CoverTrialStatus>(`/api/templates/${encodeURIComponent(templateId)}/cover-trials`, { signal })

export function startCoverTrial(templateId: string, input: { jobId: string; confirm: true }): Promise<CoverTrialStatus> {
  return mutate<CoverTrialStatus>('POST', `/api/templates/${encodeURIComponent(templateId)}/cover-trials`, input)
}

export const cancelCoverTrial = (templateId: string, trialId: string) =>
  mutate<void>('POST', `/api/templates/${encodeURIComponent(templateId)}/cover-trials/${encodeURIComponent(trialId)}/cancel`, {})

export const coverTrialImageUrl = (templateId: string, trialId: string) =>
  `/api/templates/${encodeURIComponent(templateId)}/cover-trials/${encodeURIComponent(trialId)}/image`

export const getPromptSamples = (promptId: string, signal?: AbortSignal) =>
  request<PromptSampleStatus>(`/api/image-prompts/${encodeURIComponent(promptId)}/samples`, { signal })

export function startPromptSample(promptId: string, input: { videoId: string; sceneId?: string; confirm: true }): Promise<PromptSampleStatus> {
  return mutate<PromptSampleStatus>('POST', `/api/image-prompts/${encodeURIComponent(promptId)}/samples`, input)
}

export const cancelPromptSample = (promptId: string, sampleId: string) =>
  mutate<void>('POST', `/api/image-prompts/${encodeURIComponent(promptId)}/samples/${encodeURIComponent(sampleId)}/cancel`, {})

export const promptSampleImageUrl = (promptId: string, sampleId: string) =>
  `/api/image-prompts/${encodeURIComponent(promptId)}/samples/${encodeURIComponent(sampleId)}/image`

export const getPromptTrialScenes = (promptId: string, videoId: string, signal?: AbortSignal) =>
  request<{ items: TrialScene[] }>(
    `/api/image-prompts/${encodeURIComponent(promptId)}/samples/scenes?videoId=${encodeURIComponent(videoId)}`,
    { signal },
  )
