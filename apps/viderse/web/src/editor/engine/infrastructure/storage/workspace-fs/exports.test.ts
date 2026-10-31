import { beforeEach, describe, expect, it, vi } from 'vitest'
import { saveRenderedExport } from './exports'

const mocks = vi.hoisted(() => ({ beginExport: vi.fn(), uploadExport: vi.fn(), commitExport: vi.fn(), abortExport: vi.fn(), release: vi.fn() }))
vi.mock('@editor/storage/storage-backend', () => ({ getEditorStorage: () => mocks }))
vi.mock('@editor/features/export/utils/export-output-target', () => ({ releaseTemporaryExportOutput: mocks.release }))
beforeEach(() => {
  vi.clearAllMocks()
  mocks.beginExport.mockResolvedValue('session')
  mocks.uploadExport.mockResolvedValue(undefined)
  mocks.commitExport.mockResolvedValue({ id: 'session', status: 'ready', fileName: 'out.mp4', size: 7 })
  mocks.abortExport.mockResolvedValue(undefined)
  mocks.release.mockResolvedValue(undefined)
})
const output = { blob: new Blob(['encoded']), temporaryOutput: { directory: 'freecut-export-output', fileName: 'scratch.mp4' } }
describe('export scratch lifecycle', () => {
  it('releases disk-backed scratch only after upload and commit succeed', async () => {
    await saveRenderedExport('project', 'out.mp4', output)
    expect(mocks.uploadExport).toHaveBeenCalledWith('project', 'session', output.blob, {})
    expect(mocks.release).toHaveBeenCalledWith(output)
    expect(mocks.release.mock.invocationCallOrder[0]).toBeGreaterThan(mocks.commitExport.mock.invocationCallOrder[0])
    expect(mocks.abortExport).not.toHaveBeenCalled()
  })
  it('aborts a failed session while retaining scratch for another upload attempt', async () => {
    mocks.uploadExport.mockRejectedValueOnce(new Error('network failed'))
    await expect(saveRenderedExport('project', 'out.mp4', output)).rejects.toThrow('network failed')
    expect(mocks.abortExport).toHaveBeenCalledWith('project', 'session')
    expect(mocks.release).not.toHaveBeenCalled()
    await saveRenderedExport('project', 'out.mp4', output)
    expect(mocks.release).toHaveBeenCalledTimes(1)
  })
  it('aborts cancellation without deleting the encoded output', async () => {
    const controller = new AbortController()
    controller.abort()
    await expect(saveRenderedExport('project', 'out.mp4', output, { signal: controller.signal })).rejects.toMatchObject({ name: 'AbortError' })
    expect(mocks.uploadExport).not.toHaveBeenCalled()
    expect(mocks.commitExport).not.toHaveBeenCalled()
    expect(mocks.abortExport).toHaveBeenCalledWith('project', 'session')
    expect(mocks.release).not.toHaveBeenCalled()
  })
})
