import { z } from 'zod'
import { mutate, request } from './request'

const sourceSchema = z.object({ id: z.string(), url: z.string(), title: z.string(), channelId: z.string().nullable(), channelTitle: z.string().nullable(), thumbnailUrl: z.string().nullable(), durationSec: z.number().nullable(), outlier: z.number().optional(), viewCount: z.number().optional(), publishedAt: z.string().optional(), hasCaptions: z.boolean().optional(), captionLanguage: z.string().optional(), chapterCount: z.number().optional(), transcriptExcerpt: z.string().optional() }).passthrough()
const stepSchema = z.object({ step: z.string(), status: z.string(), progress: z.object({ unitsDone: z.number(), unitsTotal: z.number().nullable(), phase: z.string() }), output: z.object({ artifacts: z.array(z.object({ id: z.string(), path: z.string(), contentType: z.string() }).passthrough()).nullable(), data: z.record(z.string(), z.unknown()).optional() }).nullable(), error: z.object({ code: z.string(), message: z.string() }).nullable(), startedAt: z.string().nullable(), finishedAt: z.string().nullable() }).passthrough()
const agentSchema = z.object({ id: z.string(), state: z.string(), now: z.string().nullable(), metric: z.object({ label: z.string(), value: z.string() }).nullable() })
export const cloneJobViewSchema = z.object({ id: z.string(), channelId: z.string(), source: sourceSchema, sourceKey: z.string(), generation: z.number(), templateId: z.string(), status: z.string(), stageGroup: z.string(), activeStep: z.string().nullable(), version: z.number(), noImages: z.boolean(), exportBytes: z.number().nullable().optional(), previousJobId: z.string().nullable().optional(), subline: z.string(), agentView: z.array(agentSchema).nullable(), steps: z.array(stepSchema).nullable(), queueItem: z.object({ overrideRevision: z.number().optional(), scenePromptId: z.string().nullable().optional(), coverPromptId: z.string().nullable().optional(), noImages: z.boolean().optional() }).passthrough().optional() }).passthrough()
export type CloneJobView = z.infer<typeof cloneJobViewSchema>
const listSchema = z.object({ jobs: z.array(cloneJobViewSchema).nullable(), queuedTotal: z.number(), defaultJobId: z.string().nullable() }).passthrough()
export const getWorkbench = async (channel: string, signal?: AbortSignal) => listSchema.parse(await request<unknown>(`/api/clone?channel=${encodeURIComponent(channel)}`, { signal }))
export const getWorkbenchJob = async (id: string, signal?: AbortSignal) => cloneJobViewSchema.parse(await request<unknown>(`/api/clone/${encodeURIComponent(id)}`, { signal }))
export const jobAction = (id: string, action: 'pause' | 'resume' | 'cancel' | 'run' | 'delete') => mutate('POST', `/api/clone/${encodeURIComponent(id)}`, { action })
const restartSchema = z.object({ restartedFrom: z.string(), queueItem: z.object({ id: z.string() }) }).passthrough()
const restartPreviewSchema = z.object({ template: z.object({ id: z.string(), name: z.string() }), imageEstimate: z.number().nullable(), position: z.literal('front'), etaStart: z.string().nullable() })
export const previewRestart = async (id: string) => restartPreviewSchema.parse(await mutate('POST', `/api/clone/${encodeURIComponent(id)}`, { action: 'restart', dryRun: true }))
export const restartJob = async (id: string) => restartSchema.parse(await mutate('POST', `/api/clone/${encodeURIComponent(id)}`, { action: 'restart' }))
export const acceptExport = (id: string) => mutate('POST', `/api/clone/${encodeURIComponent(id)}/export`, { accept: true })
const bulkResultSchema = z.object({ results: z.array(z.object({ ok: z.boolean(), error: z.string().optional() })) })
export async function jobBulkAction(id: string, action: 'posted' | 'deleteExport') {
  const result = bulkResultSchema.parse(await mutate('POST', '/api/clone/bulk', { action, jobIds: [id] }))
  if (!result.results[0]?.ok) throw new Error(result.results[0]?.error || 'Không thực hiện được')
}
const scoreSchema = z.object({ total: z.number(), blockers: z.array(z.object({ id: z.string(), message: z.string() })).nullable() }).passthrough()
export const getScoreArtifact = async (jobId: string, artifactId: string, signal?: AbortSignal) => scoreSchema.parse(await request<unknown>(`/api/clone/${encodeURIComponent(jobId)}/artifacts/${encodeURIComponent(artifactId)}`, { signal }))

export const promptPlanSchema = z.object({ redraw: z.object({ scenes: z.number(), cover: z.boolean() }), cost: z.object({ images: z.number() }), quotaLeft: z.number(), rebuild: z.boolean() })
export type PromptPlan = z.infer<typeof promptPlanSchema>
const imageSchema = z.object({ id: z.string(), kind: z.string(), status: z.string(), currentVersion: z.number().nullable(), reviewVersion: z.number().nullable(), versions: z.array(z.object({ version: z.number(), originalUrl: z.string().optional() })).nullable(), sceneId: z.string().optional(), narration: z.string().optional() }).passthrough()
const effectivePrompt = z.object({ id: z.string(), name: z.string(), overridden: z.boolean() })
const imageListSchema = z.object({ manifest: z.object({ revision: z.number(), scenePrompt: effectivePrompt, coverPrompt: effectivePrompt }).nullable(), items: z.array(imageSchema).nullable() }).passthrough()
export const getJobImages = async (id: string, signal?: AbortSignal) => imageListSchema.parse(await request<unknown>(`/api/clone/${encodeURIComponent(id)}/images`, { signal }))
export const redrawAll = (id: string, expectedRevision: number, confirmCount: number, feedback: string) => mutate('POST', `/api/clone/${encodeURIComponent(id)}/images`, { op: 'regenerateAll', expectedRevision, confirmCount, feedback })
export async function uploadCover(id: string, revision: number, file: File) {
  const response = await fetch(`/api/clone/${encodeURIComponent(id)}/cover?expectedRevision=${revision}`, { method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': file.type || 'application/octet-stream' }, body: file })
  if (!response.ok) {
    const body: unknown = await response.json().catch(() => null)
    throw new Error(body && typeof body === 'object' && 'error' in body && typeof body.error === 'string' ? body.error : 'Không tải được ảnh bìa')
  }
}
export type PromptChoice = { scenePromptId?: string | null; coverPromptId?: string | null; noImages?: boolean }
export const planPrompt = async (id: string, choice: PromptChoice) => promptPlanSchema.parse(await mutate('POST', `/api/clone/${encodeURIComponent(id)}/prompts/plan`, choice))
export const applyPrompt = (id: string, choice: PromptChoice, expectedRevision: number, confirmCount: number) => mutate('PATCH', `/api/clone/${encodeURIComponent(id)}/prompts`, { ...choice, expectedRevision, confirmCount })

const recapSchema = z.object({ from: z.string(), to: z.string(), summary: z.string(), hasNew: z.boolean(), needsYou: z.array(z.object({ jobId: z.string(), label: z.string(), state: z.string() })).nullable(), auto: z.array(z.object({ kind: z.string(), text: z.string(), count: z.number(), href: z.string() })).nullable(), suggestions: z.array(z.unknown()).nullable(), today: z.object({ queued: z.number(), expectedDone: z.number().nullable(), byTime: z.string().nullable(), imagesLeft: z.number().nullable(), imagesTotal: z.number().nullable(), paidLeft: z.number().optional() }), channels: z.array(z.object({ channelId: z.string(), done: z.number(), cap: z.number(), waiting: z.number(), ready: z.number() })).nullable() }).passthrough()
export type RecapData = z.infer<typeof recapSchema>
export const getRecap = async (signal?: AbortSignal) => recapSchema.parse(await request<unknown>('/api/recap', { signal }))
export const markRecapSeen = () => mutate('POST', '/api/recap/seen', {})
