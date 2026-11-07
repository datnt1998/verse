"use client";

/**
 * The one export pipeline: gate → begin → preflight → render → upload →
 * commit. The editor export dialog and the headless `/render` page run the
 * same code; there is no second begin/upload/commit path.
 *
 * `onExportId` fires the moment `beginExport` returns — before any rendering
 * — so a headless request can bind the staged export id to its request
 * record (crash recovery deletes staged exports by that id).
 */
import type { CompositionInputProps } from '@editor/types/export'
import type { Project } from '@editor/types/project'
import { assessExportPreflight, type ExportPreflightResult } from '@editor/features/export/utils/export-preflight'
import { runRender, type RunRenderOutcome } from '@editor/features/export/utils/render-pipeline'
import { releaseTemporaryExportOutput } from '@editor/features/export/utils/export-output-target'
import type { ExportTimings, RenderProgress } from '@editor/features/export/utils/client-renderer'
import { resolveMediaUrls } from '@editor/features/timeline/deps/media-library-resolver'
import { getEditorStorage } from '@editor/storage/storage-backend'
import type { ServerExportRecord } from '@editor/storage/types'
import { fetchMeasurementState } from '@/editor/ui/clone/measurement-runtime'
import { cloneExportSettingsAllowed } from '@/editor/ui/clone/quality-gate'

export type ServerExportProgress =
  | { kind: 'render'; progress: RenderProgress }
  | { kind: 'upload'; sent: number; total: number; timings?: ExportTimings }
  | { kind: 'publish'; timings?: ExportTimings }

export interface ServerExportResult {
  record: ServerExportRecord
  outcome: RunRenderOutcome
  fileName: string
}

/** Clone-bound export: score gate, fixed settings and revision pinning apply. */
export interface ServerExportClone {
  jobId: string
  /** UI-supplied unsaved-state gate; the server-side fresh-score check always runs. */
  canExport?: boolean
  gateReason?: string | null
}

function cancelled(signal: AbortSignal) {
  if (signal.aborted) throw new DOMException('Đã huỷ xuất', 'AbortError')
}

/**
 * Upload + commit a rendered outcome. `exportId` resumes an already-begun
 * session (clone exports begin before rendering, and headless requests bind
 * the id to their record); otherwise a session is opened here with the real
 * size for the free-space check. A failed or aborted upload aborts the
 * staged session server-side.
 */
export async function saveServerExport(options: {
  projectId: string
  fileName: string
  outcome: RunRenderOutcome
  projectRevision?: string
  clone?: ServerExportClone
  exportId?: string
  requestId?: string
  signal: AbortSignal
  onExportId?: (exportId: string) => void | Promise<void>
  onProgress?: (progress: ServerExportProgress) => void
}): Promise<ServerExportResult> {
  const { outcome, projectId, fileName, projectRevision, signal } = options
  const storage = getEditorStorage()
  const timings = outcome.result.timings ? { ...outcome.result.timings } : undefined
  let sentBytes = 0
  let totalBytes = outcome.result.blob.size
  const reportUpload = () => options.onProgress?.({
    kind: 'upload', sent: sentBytes, total: totalBytes,
    ...(timings ? { timings: { ...timings } } : {}),
  })
  reportUpload()
  let exportId = options.exportId ?? null
  try {
    cancelled(signal)
    if (exportId === null) {
      const cloneContext = options.clone ? { clone: { projectRevision } } : {}
      exportId = await storage.beginExport(projectId, {
        fileName,
        mime: outcome.result.mimeType,
        expectedSize: outcome.result.blob.size,
        requestId: options.requestId,
        ...cloneContext,
      })
      await options.onExportId?.(exportId)
    }
    cancelled(signal)
    const uploadStart = performance.now()
    try {
      await storage.uploadExport(projectId, exportId, outcome.result.blob, {
        signal,
        mime: outcome.result.mimeType,
        onProgress: (sent, total) => {
          sentBytes = sent
          totalBytes = total
          if (timings) timings.uploadMs = performance.now() - uploadStart
          reportUpload()
        },
      })
    } finally {
      if (timings) timings.uploadMs = performance.now() - uploadStart
      reportUpload()
    }
    cancelled(signal)
    const metadata = options.clone && projectRevision && outcome.result.captionCoverage
      ? { captionCoverage: { projectRevision, coverage: outcome.result.captionCoverage } }
      : undefined
    options.onProgress?.({ kind: 'publish', ...(timings ? { timings: { ...timings } } : {}) })
    const publishStart = performance.now()
    let record: ServerExportRecord
    try {
      record = await storage.commitExport(projectId, exportId, metadata)
    } finally {
      if (timings) timings.publishMs = performance.now() - publishStart
      options.onProgress?.({ kind: 'publish', ...(timings ? { timings: { ...timings } } : {}) })
    }
    cancelled(signal)
    await releaseTemporaryExportOutput(outcome.result)
    return { record, outcome, fileName }
  } catch (error) {
    const cleanup = await Promise.allSettled([
      ...(exportId !== null ? [storage.abortExport(projectId, exportId)] : []),
      releaseTemporaryExportOutput(outcome.result),
    ])
    const failures = cleanup.filter((result): result is PromiseRejectedResult => result.status === 'rejected')
    if (failures.length) throw new AggregateError([error, ...failures.map(result => result.reason)], 'Không thể dọn tệp xuất sau lỗi', { cause: error })
    throw error
  }
}

/**
 * Full export: clone gate checks (fresh server-side score + fixed settings),
 * an early `beginExport` for clone exports (the session is bound to the
 * rendered revision before a frame is drawn), preflight, render, then the
 * upload/commit of `saveServerExport`.
 */
export async function runServerExport(options: {
  project: Pick<Project, 'id' | 'name'>
  /** Saved-project revision (from `loadProject`/`getProjectRevision`; may be absent). */
  revision?: string
  composition: CompositionInputProps
  settings: Parameters<typeof assessExportPreflight>[0]['settings']
  fps: number
  durationFrames: number
  clone?: ServerExportClone
  requestId?: string
  /** Headless callers read the gate through their request-scoped token, never a user session. */
  fetchCloneGate?: () => Promise<{ allowed: boolean }>
  brokenMediaIds?: string[]
  signal: AbortSignal
  onExportId?: (exportId: string) => void | Promise<void>
  onPreflight?: (result: ExportPreflightResult) => void
  onRendered?: (outcome: RunRenderOutcome) => void
  onProgress?: (progress: ServerExportProgress) => void
}): Promise<ServerExportResult> {
  const { project, revision, settings, composition, signal, clone } = options
  const storage = getEditorStorage()

  if (clone) {
    if (clone.canExport === false) throw new Error(clone.gateReason ?? 'Độ hoàn thiện chưa đạt — đo lại trước khi xuất.')
    // The score measured the whole 1080p30 timeline; any other range/size/fps would export unmeasured output.
    if (!cloneExportSettingsAllowed({ resolution: settings.resolution, fps: settings.fps, renderWholeProject: settings.renderWholeProject })) {
      throw new Error('Video clone phải xuất toàn bộ timeline ở 1080p, 30 fps.')
    }
    const gate = options.fetchCloneGate
      ? await options.fetchCloneGate()
      : (await fetchMeasurementState(clone.jobId)).gate
    if (!gate.allowed) throw new Error('Điểm Độ hoàn thiện đã cũ hoặc chưa đạt — đo lại.')
  }

  const fileName = `${project.name.replace(/[\\/:*?"<>|]/g, '-').trim() || 'video'}.mp4`

  // Clone exports begin before render: the staged session pins the revision
  // so a mid-render save is caught at commit, and headless requests bind the
  // export id to their record before the first frame.
  let exportId: string | null = null
  let renderedOutcome: RunRenderOutcome | undefined
  try {
    if (clone) {
      exportId = await storage.beginExport(project.id, {
        fileName,
        mime: 'video/mp4',
        clone: { projectRevision: revision },
        requestId: options.requestId,
      })
      await options.onExportId?.(exportId)
    }

    const resolvedComposition = { ...composition, tracks: await resolveMediaUrls(composition.tracks, { useProxy: false, signal }) }
    cancelled(signal)
    const checked = await assessExportPreflight({
      settings,
      fps: options.fps,
      composition: resolvedComposition,
      durationFrames: options.durationFrames,
      brokenMediaIds: options.brokenMediaIds,
    })
    options.onPreflight?.(checked)
    if (!checked.canExport || !checked.resolvedSettings) {
      throw new Error('Kiểm tra trước khi xuất chưa đạt. Xem chi tiết và sửa project.')
    }
    const outcome = await runRender({
      clientSettings: { ...checked.resolvedSettings, collectCaptionCoverage: !!clone },
      exportMode: 'video',
      composition: resolvedComposition,
      signal,
      onProgress: (progress) => options.onProgress?.({ kind: 'render', progress }),
    })
    renderedOutcome = outcome
    options.onRendered?.(outcome)
    cancelled(signal)
    // Upload owns cleanup from this point; do not abort the same session twice.
    const stagedExportId = exportId
    exportId = null
    renderedOutcome = undefined
    return await saveServerExport({
      projectId: project.id,
      fileName,
      outcome,
      projectRevision: revision,
      clone,
      exportId: stagedExportId ?? undefined,
      requestId: options.requestId,
      signal,
      onExportId: options.onExportId,
      onProgress: options.onProgress,
    })
  } catch (error) {
    const cleanup = await Promise.allSettled([
      ...(exportId !== null ? [storage.abortExport(project.id, exportId)] : []),
      ...(renderedOutcome ? [releaseTemporaryExportOutput(renderedOutcome.result)] : []),
    ])
    const failures = cleanup.filter((result): result is PromiseRejectedResult => result.status === 'rejected')
    if (failures.length) throw new AggregateError([error, ...failures.map(result => result.reason)], 'Không thể dọn phiên xuất sau lỗi', { cause: error })
    throw error
  }
}
