import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { EditorStorage } from '@editor/storage/storage-backend'
import type { RunRenderOutcome } from '@editor/features/export/utils/render-pipeline'
import { cloneExportSettings } from './export-settings'
import { runServerExport, saveServerExport } from './export-runner'

const dependencies = vi.hoisted(() => ({
  storage: { beginExport: vi.fn(), uploadExport: vi.fn(), commitExport: vi.fn(), abortExport: vi.fn() },
  gate: vi.fn(), preflight: vi.fn(), render: vi.fn(), release: vi.fn(),
}))
vi.mock('@editor/storage/storage-backend', () => ({ getEditorStorage: () => dependencies.storage satisfies Pick<EditorStorage, 'beginExport' | 'uploadExport' | 'commitExport' | 'abortExport'> }))
vi.mock('@editor/features/export/utils/export-preflight', () => ({ assessExportPreflight: dependencies.preflight }))
vi.mock('@editor/features/export/utils/render-pipeline', () => ({ runRender: dependencies.render }))
vi.mock('@editor/features/export/utils/export-output-target', () => ({ releaseTemporaryExportOutput: dependencies.release }))
vi.mock('@editor/features/timeline/deps/media-library-resolver', () => ({ resolveMediaUrls: async (tracks: unknown) => tracks }))
vi.mock('../clone/measurement-runtime', () => ({ fetchMeasurementState: dependencies.gate }))
vi.mock('../clone/quality-gate', () => ({ cloneExportSettingsAllowed: () => true }))

function outcome(): RunRenderOutcome {
  return { renderPath: 'worker', result: { blob: new Blob(['mp4']), mimeType: 'video/mp4', duration: 1, fileSize: 3 } }
}
const signal = () => new AbortController().signal

beforeEach(() => {
  dependencies.storage.beginExport.mockResolvedValue('export-1')
  dependencies.storage.uploadExport.mockResolvedValue(undefined)
  dependencies.storage.commitExport.mockResolvedValue({ id: 'export-1', fileName: 'Video.mp4', mime: 'video/mp4', size: 3, status: 'ready', createdAt: 1, committedAt: 2 })
  dependencies.storage.abortExport.mockResolvedValue(undefined)
  dependencies.preflight.mockResolvedValue({ canExport: true, resolvedSettings: {} })
  dependencies.render.mockResolvedValue(outcome())
  dependencies.release.mockResolvedValue(undefined)
})
afterEach(() => { vi.resetAllMocks() })

describe('shared export runner scoped integration', () => {
  it('uses the request-scoped gate instead of the authenticated clone endpoint', async () => {
    const scopedGate = vi.fn().mockResolvedValue({ allowed: true })
    const bind = vi.fn()
    await runServerExport({ project: { id: 'project-1', name: 'Video' }, revision: 'revision-1', composition: { fps: 30, tracks: [] }, settings: cloneExportSettings(), fps: 30, durationFrames: 30, clone: { jobId: 'job-1' }, fetchCloneGate: scopedGate, onExportId: bind, signal: signal() })
    expect(scopedGate).toHaveBeenCalledOnce()
    expect(dependencies.gate).not.toHaveBeenCalled()
    expect(bind).toHaveBeenCalledWith('export-1')
    expect(dependencies.storage.uploadExport).toHaveBeenCalledOnce()
    expect(dependencies.release).toHaveBeenCalledOnce()
  })

  it('aborts the staged export if binding fails before rendering', async () => {
    const failure = new Error('fence expired')
    await expect(runServerExport({ project: { id: 'project-1', name: 'Video' }, revision: 'revision-1', composition: { fps: 30, tracks: [] }, settings: cloneExportSettings(), fps: 30, durationFrames: 30, clone: { jobId: 'job-1' }, fetchCloneGate: async () => ({ allowed: true }), onExportId: async () => { throw failure }, signal: signal() })).rejects.toBe(failure)
    expect(dependencies.storage.abortExport).toHaveBeenCalledExactlyOnceWith('project-1', 'export-1')
    expect(dependencies.render).not.toHaveBeenCalled()
  })

  it('releases temporary render output and aborts once when upload fails', async () => {
    const failure = new Error('upload failed')
    dependencies.storage.uploadExport.mockRejectedValue(failure)
    await expect(runServerExport({ project: { id: 'project-1', name: 'Video' }, revision: 'revision-1', composition: { fps: 30, tracks: [] }, settings: cloneExportSettings(), fps: 30, durationFrames: 30, clone: { jobId: 'job-1' }, fetchCloneGate: async () => ({ allowed: true }), signal: signal() })).rejects.toBe(failure)
    expect(dependencies.storage.abortExport).toHaveBeenCalledExactlyOnceWith('project-1', 'export-1')
    expect(dependencies.release).toHaveBeenCalledOnce()
    expect(dependencies.storage.commitExport).not.toHaveBeenCalled()
  })

  it('releases rendered output when cancellation arrives before upload handoff', async () => {
    const controller = new AbortController()
    await expect(runServerExport({ project: { id: 'project-1', name: 'Video' }, revision: 'revision-1', composition: { fps: 30, tracks: [] }, settings: cloneExportSettings(), fps: 30, durationFrames: 30, clone: { jobId: 'job-1' }, fetchCloneGate: async () => ({ allowed: true }), signal: controller.signal, onRendered: () => controller.abort() })).rejects.toMatchObject({ name: 'AbortError' })
    expect(dependencies.release).toHaveBeenCalledOnce()
    expect(dependencies.storage.abortExport).toHaveBeenCalledExactlyOnceWith('project-1', 'export-1')
    expect(dependencies.storage.uploadExport).not.toHaveBeenCalled()
  })

  it('keeps cleanup failures alongside the original upload error', async () => {
    const failure = new Error('upload failed')
    const cleanup = new Error('abort failed')
    dependencies.storage.uploadExport.mockRejectedValue(failure)
    dependencies.storage.abortExport.mockRejectedValue(cleanup)
    await expect(saveServerExport({ projectId: 'project-1', exportId: 'export-1', fileName: 'Video.mp4', outcome: outcome(), signal: signal() })).rejects.toMatchObject({ cause: failure, errors: [failure, cleanup] })
    expect(dependencies.release).toHaveBeenCalledOnce()
  })
})
