// Kiểu wire của Bàn dựng (GET /api/clone…) — hợp đồng 07-data-contract §7.
//
// Đọc bằng guard tay (./guard) chứ không bằng zod: đây là đường đọc của màn đầu
// điện thoại (Bàn dựng) và của pane giữ-alive, nên không được kéo cả thư viện
// schema vào entry.
import { bool, list, maybe, num, oneOf, record, str } from './guard'
import { mutate, request } from './request'
import { readTemplateSharedAssets, type TemplateSharedAsset } from './templateAssets'

export type JobSource = {
  id: string
  url: string
  title: string
  channelId: string | null
  channelTitle: string | null
  thumbnailUrl: string | null
  durationSec: number | null
  outlier?: number
  viewCount?: number
  publishedAt?: string
  hasCaptions?: boolean
  captionLanguage?: string
  chapterCount?: number
  transcriptExcerpt?: string
}

export type JobArtifact = { id: string; path: string; contentType: string }

export type JobStep = {
  step: string
  status: string
  progress: { unitsDone: number; unitsTotal: number | null; phase: string }
  output: { artifacts: JobArtifact[] | null; data?: Record<string, unknown> } | null
  error: { code: string; message: string } | null
  startedAt: string | null
  finishedAt: string | null
}

export type JobAgent = { id: string; state: string; now: string | null; metric: { label: string; value: string } | null }

export type CloneJobQueueItem = {
  overrideRevision?: number
  scenePromptId?: string | null
  coverPromptId?: string | null
  noImages?: boolean
}

export type CloneJobView = {
  id: string
  channelId: string
  source: JobSource
  sourceKey: string
  generation: number
  supersededAt?: string | null
  templateId: string
  /** Tên template lúc tạo job (library.Snapshot.templateName). */
  templateName?: string
  templateSharedAssets?: TemplateSharedAsset[]
  status: string
  stageGroup: string
  activeStep: string | null
  version: number
  noImages: boolean
  exportBytes?: number | null
  previousJobId?: string | null
  subline: string
  agentView: JobAgent[] | null
  steps: JobStep[] | null
  queueItem?: CloneJobQueueItem
}

const nullableStr = (value: unknown, path: string) => (value === null ? null : str(value, path))
/** `.nullable().optional()` của zod cũ: vắng mặt → undefined, null giữ nguyên null. */
const nullishStr = (value: unknown, path: string) => (value == null ? undefined : str(value, path))
const nullishNum = (value: unknown, path: string) => (value == null ? undefined : num(value, path))

const readSource = (value: unknown, path: string): JobSource => {
  const o = record(value, path)
  return {
    id: str(o.id, `${path}.id`),
    url: str(o.url, `${path}.url`),
    title: str(o.title, `${path}.title`),
    channelId: nullableStr(o.channelId, `${path}.channelId`),
    channelTitle: nullableStr(o.channelTitle, `${path}.channelTitle`),
    thumbnailUrl: nullableStr(o.thumbnailUrl, `${path}.thumbnailUrl`),
    durationSec: o.durationSec === null ? null : num(o.durationSec, `${path}.durationSec`),
    ...(o.outlier == null ? {} : { outlier: num(o.outlier, `${path}.outlier`) }),
    ...(o.viewCount == null ? {} : { viewCount: num(o.viewCount, `${path}.viewCount`) }),
    ...(o.publishedAt == null ? {} : { publishedAt: str(o.publishedAt, `${path}.publishedAt`) }),
    ...(o.hasCaptions == null ? {} : { hasCaptions: bool(o.hasCaptions, `${path}.hasCaptions`) }),
    ...(o.captionLanguage == null ? {} : { captionLanguage: str(o.captionLanguage, `${path}.captionLanguage`) }),
    ...(o.chapterCount == null ? {} : { chapterCount: num(o.chapterCount, `${path}.chapterCount`) }),
    ...(o.transcriptExcerpt == null ? {} : { transcriptExcerpt: str(o.transcriptExcerpt, `${path}.transcriptExcerpt`) }),
  }
}

const readStep = (value: unknown, path: string): JobStep => {
  const o = record(value, path)
  const progress = record(o.progress, `${path}.progress`)
  const output = o.output == null ? null : (() => {
    const out = record(o.output, `${path}.output`)
    return {
      artifacts: out.artifacts == null ? null : list(out.artifacts, (entry, at) => {
        const artifact = record(entry, at)
        return { id: str(artifact.id, `${at}.id`), path: str(artifact.path, `${at}.path`), contentType: str(artifact.contentType, `${at}.contentType`) }
      }, `${path}.output.artifacts`),
      ...(out.data == null ? {} : { data: record(out.data, `${path}.output.data`) }),
    }
  })()
  const error = o.error == null ? null : (() => {
    const err = record(o.error, `${path}.error`)
    return { code: str(err.code, `${path}.error.code`), message: str(err.message, `${path}.error.message`) }
  })()
  return {
    step: str(o.step, `${path}.step`),
    status: str(o.status, `${path}.status`),
    progress: {
      unitsDone: num(progress.unitsDone, `${path}.progress.unitsDone`),
      unitsTotal: progress.unitsTotal === null ? null : num(progress.unitsTotal, `${path}.progress.unitsTotal`),
      phase: str(progress.phase, `${path}.progress.phase`),
    },
    output,
    error,
    startedAt: nullableStr(o.startedAt, `${path}.startedAt`),
    finishedAt: nullableStr(o.finishedAt, `${path}.finishedAt`),
  }
}

const readAgent = (value: unknown, path: string): JobAgent => {
  const o = record(value, path)
  return {
    id: str(o.id, `${path}.id`),
    state: str(o.state, `${path}.state`),
    now: nullableStr(o.now, `${path}.now`),
    metric: o.metric == null ? null : (() => {
      const metric = record(o.metric, `${path}.metric`)
      return { label: str(metric.label, `${path}.metric.label`), value: str(metric.value, `${path}.metric.value`) }
    })(),
  }
}

const readQueueItem = (value: unknown, path: string): CloneJobQueueItem => {
  const o = record(value, path)
  const overrideRevision = nullishNum(o.overrideRevision, `${path}.overrideRevision`)
  const scenePromptId = o.scenePromptId === null ? null : nullishStr(o.scenePromptId, `${path}.scenePromptId`)
  const coverPromptId = o.coverPromptId === null ? null : nullishStr(o.coverPromptId, `${path}.coverPromptId`)
  const noImages = o.noImages == null ? undefined : bool(o.noImages, `${path}.noImages`)
  return {
    ...(overrideRevision === undefined ? {} : { overrideRevision }),
    ...(scenePromptId === undefined ? {} : { scenePromptId }),
    ...(coverPromptId === undefined ? {} : { coverPromptId }),
    ...(noImages === undefined ? {} : { noImages }),
  }
}

/** Một job của Bàn dựng; `path` chỉ để thông điệp lỗi đọc được. */
export const readCloneJob = (value: unknown, path: string): CloneJobView => {
  const o = record(value, path)
  const supersededAt = o.supersededAt === null ? null : nullishStr(o.supersededAt, `${path}.supersededAt`)
  const exportBytes = o.exportBytes === null ? null : nullishNum(o.exportBytes, `${path}.exportBytes`)
  const previousJobId = o.previousJobId === null ? null : nullishStr(o.previousJobId, `${path}.previousJobId`)
  const template = o.templateSnapshot == null ? undefined : record(o.templateSnapshot, `${path}.templateSnapshot`)
  return {
    id: str(o.id, `${path}.id`),
    channelId: str(o.channelId, `${path}.channelId`),
    source: readSource(o.source, `${path}.source`),
    sourceKey: str(o.sourceKey, `${path}.sourceKey`),
    generation: num(o.generation, `${path}.generation`),
    ...(supersededAt === undefined ? {} : { supersededAt }),
    templateId: str(o.templateId, `${path}.templateId`),
    ...(template === undefined ? {} : { templateName: maybe(template.templateName, str, `${path}.templateSnapshot.templateName`), templateSharedAssets: readTemplateSharedAssets(template, `${path}.templateSnapshot`) }),
    status: str(o.status, `${path}.status`),
    stageGroup: str(o.stageGroup, `${path}.stageGroup`),
    activeStep: nullableStr(o.activeStep, `${path}.activeStep`),
    version: num(o.version, `${path}.version`),
    noImages: bool(o.noImages, `${path}.noImages`),
    ...(exportBytes === undefined ? {} : { exportBytes }),
    ...(previousJobId === undefined ? {} : { previousJobId }),
    subline: str(o.subline, `${path}.subline`),
    agentView: o.agentView == null ? null : list(o.agentView, readAgent, `${path}.agentView`),
    steps: o.steps == null ? null : list(o.steps, readStep, `${path}.steps`),
    ...(o.queueItem == null ? {} : { queueItem: readQueueItem(o.queueItem, `${path}.queueItem`) }),
  }
}

export type WorkbenchList = { jobs: CloneJobView[] | null; queuedTotal: number; defaultJobId: string | null }

export const readWorkbenchList = (value: unknown): WorkbenchList => {
  const o = record(value, 'workbench')
  return {
    jobs: o.jobs == null ? null : list(o.jobs, readCloneJob, 'workbench.jobs'),
    queuedTotal: num(o.queuedTotal, 'workbench.queuedTotal'),
    defaultJobId: nullableStr(o.defaultJobId, 'workbench.defaultJobId'),
  }
}

export const getWorkbench = async (channel: string, signal?: AbortSignal) => readWorkbenchList(await request<unknown>(`/api/clone?channel=${encodeURIComponent(channel)}`, { signal }))
export const getWorkbenchJob = async (id: string, signal?: AbortSignal) => readCloneJob(await request<unknown>(`/api/clone/${encodeURIComponent(id)}`, { signal }), 'job')
export const jobAction = (id: string, action: 'pause' | 'resume' | 'cancel' | 'run') => mutate('POST', `/api/clone/${encodeURIComponent(id)}`, { action })
/** Xoá video bằng HTTP DELETE — action switch của máy chủ không nhận `delete`. */
export const deleteJob = (id: string) => mutate<void>('DELETE', `/api/clone/${encodeURIComponent(id)}`)

export type RestartPreview = { template: { id: string; name: string }; imageEstimate: number | null; position: 'front'; etaStart: string | null }

const readRestartPreview = (value: unknown): RestartPreview => {
  const o = record(value, 'restartPreview')
  const template = record(o.template, 'restartPreview.template')
  return {
    template: { id: str(template.id, 'restartPreview.template.id'), name: str(template.name, 'restartPreview.template.name') },
    imageEstimate: o.imageEstimate === null ? null : num(o.imageEstimate, 'restartPreview.imageEstimate'),
    position: oneOf(o.position, ['front'] as const, 'restartPreview.position'),
    etaStart: nullableStr(o.etaStart, 'restartPreview.etaStart'),
  }
}

const readRestart = (value: unknown): { restartedFrom: string; queueItem: { id: string } } => {
  const o = record(value, 'restart')
  const queueItem = record(o.queueItem, 'restart.queueItem')
  return { restartedFrom: str(o.restartedFrom, 'restart.restartedFrom'), queueItem: { id: str(queueItem.id, 'restart.queueItem.id') } }
}

export const previewRestart = async (id: string) => readRestartPreview(await mutate('POST', `/api/clone/${encodeURIComponent(id)}`, { action: 'restart', dryRun: true }))
export const restartJob = async (id: string) => readRestart(await mutate('POST', `/api/clone/${encodeURIComponent(id)}`, { action: 'restart' }))
export const acceptExport = (id: string) => mutate('POST', `/api/clone/${encodeURIComponent(id)}/export`, { accept: true })

const readBulkResult = (value: unknown) => {
  const o = record(value, 'bulk')
  return list(o.results, (entry, at) => {
    const row = record(entry, at)
    return { ok: bool(row.ok, `${at}.ok`), ...(row.error == null ? {} : { error: str(row.error, `${at}.error`) }) }
  }, 'bulk.results')
}

export async function jobBulkAction(id: string, action: 'posted' | 'deleteExport') {
  const results = readBulkResult(await mutate('POST', '/api/clone/bulk', { action, jobIds: [id] }))
  if (!results[0]?.ok) throw new Error(results[0]?.error || 'Không thực hiện được')
}

export type ScoreArtifact = { total: number; blockers: { id: string; message: string }[] | null }

export const getScoreArtifact = async (jobId: string, artifactId: string, signal?: AbortSignal): Promise<ScoreArtifact> => {
  const o = record(await request<unknown>(`/api/clone/${encodeURIComponent(jobId)}/artifacts/${encodeURIComponent(artifactId)}`, { signal }), 'score')
  return {
    total: num(o.total, 'score.total'),
    blockers: o.blockers == null ? null : list(o.blockers, (entry, at) => {
      const blocker = record(entry, at)
      return { id: str(blocker.id, `${at}.id`), message: str(blocker.message, `${at}.message`) }
    }, 'score.blockers'),
  }
}

export type PromptPlan = { redraw: { scenes: number; cover: boolean }; cost: { images: number }; quotaLeft: number; rebuild: boolean; enablesImages: boolean }

const readPromptPlan = (value: unknown): PromptPlan => {
  const o = record(value, 'promptPlan')
  const redraw = record(o.redraw, 'promptPlan.redraw')
  const cost = record(o.cost, 'promptPlan.cost')
  return {
    redraw: { scenes: num(redraw.scenes, 'promptPlan.redraw.scenes'), cover: bool(redraw.cover, 'promptPlan.redraw.cover') },
    cost: { images: num(cost.images, 'promptPlan.cost.images') },
    quotaLeft: num(o.quotaLeft, 'promptPlan.quotaLeft'),
    rebuild: bool(o.rebuild, 'promptPlan.rebuild'),
    enablesImages: bool(o.enablesImages, 'promptPlan.enablesImages'),
  }
}

export type JobImage = {
  id: string
  kind: string
  status: string
  currentVersion: number | null
  reviewVersion: number | null
  versions: { version: number; originalUrl?: string; prompt?: string }[] | null
  sceneId?: string
  startSec?: number
  endSec?: number
  narration?: string
  prompt?: string
}

export type EffectivePrompt = { id: string; name: string; overridden: boolean }

export type JobImageList = {
  manifest: { revision: number; scenePrompt: EffectivePrompt; coverPrompt: EffectivePrompt } | null
  items: JobImage[] | null
}

const readImage = (value: unknown, path: string): JobImage => {
  const o = record(value, path)
  return {
    id: str(o.id, `${path}.id`),
    kind: str(o.kind, `${path}.kind`),
    status: str(o.status, `${path}.status`),
    currentVersion: o.currentVersion === null ? null : num(o.currentVersion, `${path}.currentVersion`),
    reviewVersion: o.reviewVersion === null ? null : num(o.reviewVersion, `${path}.reviewVersion`),
    versions: o.versions == null ? null : list(o.versions, (entry, at) => {
      const version = record(entry, at)
      return {
        version: num(version.version, `${at}.version`),
        ...(version.originalUrl == null ? {} : { originalUrl: str(version.originalUrl, `${at}.originalUrl`) }),
        ...(version.prompt == null ? {} : { prompt: str(version.prompt, `${at}.prompt`) }),
      }
    }, `${path}.versions`),
    ...(o.sceneId == null ? {} : { sceneId: str(o.sceneId, `${path}.sceneId`) }),
    ...(o.startSec == null ? {} : { startSec: num(o.startSec, `${path}.startSec`) }),
    ...(o.endSec == null ? {} : { endSec: num(o.endSec, `${path}.endSec`) }),
    ...(o.narration == null ? {} : { narration: str(o.narration, `${path}.narration`) }),
    ...(o.prompt == null ? {} : { prompt: str(o.prompt, `${path}.prompt`) }),
  }
}

const readEffectivePrompt = (value: unknown, path: string): EffectivePrompt => {
  const o = record(value, path)
  return { id: str(o.id, `${path}.id`), name: str(o.name, `${path}.name`), overridden: bool(o.overridden, `${path}.overridden`) }
}

const readImageList = (value: unknown): JobImageList => {
  const o = record(value, 'images')
  const manifest = o.manifest == null ? null : (() => {
    const m = record(o.manifest, 'images.manifest')
    return {
      revision: num(m.revision, 'images.manifest.revision'),
      scenePrompt: readEffectivePrompt(m.scenePrompt, 'images.manifest.scenePrompt'),
      coverPrompt: readEffectivePrompt(m.coverPrompt, 'images.manifest.coverPrompt'),
    }
  })()
  return { manifest, items: o.items == null ? null : list(o.items, readImage, 'images.items') }
}

export const getJobImages = async (id: string, signal?: AbortSignal) => readImageList(await request<unknown>(`/api/clone/${encodeURIComponent(id)}/images`, { signal }))
export const redrawAll = (id: string, expectedRevision: number, confirmCount: number, feedback: string) => mutate('POST', `/api/clone/${encodeURIComponent(id)}/images`, { op: 'regenerateAll', expectedRevision, confirmCount, feedback })
export const redrawImage = (id: string, imageId: string, expectedRevision: number, prompt: string) => mutate('POST', `/api/clone/${encodeURIComponent(id)}/images/${encodeURIComponent(imageId)}`, { op: 'regenerate', expectedRevision, ...(prompt ? { prompt } : {}) })
export async function uploadCover(id: string, revision: number, file: File) {
  const response = await fetch(`/api/clone/${encodeURIComponent(id)}/cover?expectedRevision=${revision}`, { method: 'POST', credentials: 'same-origin', headers: { 'Content-Type': file.type || 'application/octet-stream' }, body: file })
  if (!response.ok) {
    const body: unknown = await response.json().catch(() => null)
    throw new Error(body && typeof body === 'object' && 'error' in body && typeof body.error === 'string' ? body.error : 'Không tải được ảnh bìa')
  }
}
export type PromptChoice = { scenePromptId?: string | null; coverPromptId?: string | null; noImages?: boolean }
export const planPrompt = async (id: string, choice: PromptChoice) => readPromptPlan(await mutate('POST', `/api/clone/${encodeURIComponent(id)}/prompts/plan`, choice))
export const applyPrompt = (id: string, choice: PromptChoice, expectedRevision: number, confirmCount: number) => mutate('PATCH', `/api/clone/${encodeURIComponent(id)}/prompts`, { ...choice, expectedRevision, confirmCount })
