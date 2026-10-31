// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from 'vitest'
import '../test-utils/logger-test-mocks'

// vi.mock is hoisted above static imports, so the shared factory can only be
// loaded lazily from inside it.
vi.mock('@editor/storage/storage-backend', async () =>
  (await import('./__tests__/in-memory-opfs')).storageBackendMock,
)

import { memoryOpfs } from './__tests__/in-memory-opfs'
import { sweepWorkspaceOrphans } from './orphan-sweep'

async function plant(mediaId: string, hash: string, ...segments: string[]): Promise<void> {
  await memoryOpfs.plantText(memoryOpfs.mediaPathFor(mediaId, hash, ...segments), 'x')
}

beforeEach(() => {
  memoryOpfs.reset()
})

describe('sweepWorkspaceOrphans', () => {
  it('returns zero orphans for an empty cache', async () => {
    const report = await sweepWorkspaceOrphans()

    expect(report.totalRemoved).toBe(0)
    expect(report.liveMediaCount).toBe(0)
  })

  it('counts live media from the server and keeps their current-hash caches', async () => {
    memoryOpfs.setMedia('m1', 'h1')
    memoryOpfs.setMedia('m2', 'h2')
    await plant('m1', 'h1', 'cache', 'filmstrip', 'meta.json')
    await plant('m2', 'h2', 'cache', 'waveform', 'multi-res.bin')

    const report = await sweepWorkspaceOrphans()

    expect(report.liveMediaCount).toBe(2)
    expect(report.totalRemoved).toBe(0)
    expect(await memoryOpfs.list(memoryOpfs.mediaPathFor('m1', 'h1', 'cache'))).toEqual([
      'filmstrip',
    ])
  })

  it('removes the cache of media the server no longer has', async () => {
    memoryOpfs.setMedia('alive', 'h1')
    await plant('alive', 'h1', 'cache', 'a.bin')
    await plant('deleted', 'h9', 'cache', 'b.bin')

    const report = await sweepWorkspaceOrphans()

    expect(report.totalRemoved).toBe(1)
    expect(await memoryOpfs.list(memoryOpfs.pathFor('media'))).toEqual(['alive'])
  })

  it('removes content-hash versions that a source replacement left behind', async () => {
    memoryOpfs.setMedia('m1', 'new-hash')
    await plant('m1', 'old-hash', 'cache', 'a.bin')
    await plant('m1', 'new-hash', 'cache', 'a.bin')

    const report = await sweepWorkspaceOrphans()

    expect(report.totalRemoved).toBe(1)
    expect(await memoryOpfs.list(memoryOpfs.pathFor('media', 'm1'))).toEqual(['new-hash'])
  })

  it('dry-run reports without touching the cache', async () => {
    memoryOpfs.setMedia('m1', 'new-hash')
    await plant('m1', 'old-hash', 'cache', 'a.bin')
    await plant('gone', 'h', 'cache', 'a.bin')

    const report = await sweepWorkspaceOrphans({ dryRun: true })

    expect(report.dryRun).toBe(true)
    expect(report.totalRemoved).toBe(2)
    expect(await memoryOpfs.list(memoryOpfs.pathFor('media', 'm1'))).toEqual(['old-hash'])
    expect(await memoryOpfs.list(memoryOpfs.pathFor('media'))).toContain('gone')
  })
})
