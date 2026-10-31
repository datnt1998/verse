// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@editor/shared/logging/logger', () => ({
  createLogger: () => ({
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
    debug: vi.fn(),
    event: vi.fn(),
    startEvent: () => ({ set: vi.fn(), merge: vi.fn(), success: vi.fn(), failure: vi.fn() }),
    child: vi.fn(),
    setLevel: vi.fn(),
  }),
  createOperationId: () => 'op-test',
}))

// vi.mock is hoisted above static imports, so the shared factory can only be
// loaded lazily from inside it.
vi.mock('@editor/storage/storage-backend', async () =>
  (await import('./__tests__/in-memory-opfs')).storageBackendMock,
)

import { CACHE_ROOT } from '@editor/storage/opfs-cache'
import { memoryOpfs } from './__tests__/in-memory-opfs'
import { resetMediaIdentityTracking } from './cache-identity'
import {
  exists,
  listDirectory,
  readArrayBuffer,
  readBlob,
  readDirectoryFiles,
  readJson,
  removeEntry,
  removeFromEveryMedia,
  writeBlob,
  writeJsonAtomic,
  WorkspaceFileCorruptError,
} from './fs-primitives'

const MEDIA_FILE = ['media', 'm1', 'cache', 'data.json']

beforeEach(() => {
  memoryOpfs.reset()
  resetMediaIdentityTracking()
})

describe('fs-primitives readJson', () => {
  it('returns null for a missing file', async () => {
    expect(await readJson(CACHE_ROOT, ['nonexistent.json'])).toBeNull()
  })

  it('throws WorkspaceFileCorruptError on invalid JSON', async () => {
    await memoryOpfs.plantText(memoryOpfs.pathFor('bad.json'), '{not json')
    await expect(readJson(CACHE_ROOT, ['bad.json'])).rejects.toThrow(WorkspaceFileCorruptError)
  })

  it('WorkspaceFileCorruptError includes the path and a SyntaxError cause', async () => {
    await memoryOpfs.plantText(memoryOpfs.pathFor('corrupt.json'), '{not json')

    let caught: unknown
    try {
      await readJson(CACHE_ROOT, ['corrupt.json'])
    } catch (error) {
      caught = error
    }
    expect(caught).toBeInstanceOf(WorkspaceFileCorruptError)
    const err = caught as WorkspaceFileCorruptError
    expect(err.path).toBe('corrupt.json')
    expect(err.cause).toBeInstanceOf(SyntaxError)
  })

  it('returns parsed object for valid JSON', async () => {
    await writeJsonAtomic(CACHE_ROOT, ['nested', 'valid.json'], { hello: 'world' })
    expect(await readJson<{ hello: string }>(CACHE_ROOT, ['nested', 'valid.json'])).toEqual({
      hello: 'world',
    })
  })

  it('stores non-media paths under the workspace-scoped cache directory', async () => {
    await writeJsonAtomic(CACHE_ROOT, ['content', 'refs.json'], { a: 1 })
    expect(await memoryOpfs.readText(memoryOpfs.pathFor('content', 'refs.json'))).toContain('"a": 1')
  })
})

describe('fs-primitives content-hash versioning', () => {
  it('files a media path under the current server content hash', async () => {
    memoryOpfs.setMedia('m1', 'h1')
    await writeJsonAtomic(CACHE_ROOT, MEDIA_FILE, { v: 1 })
    expect(
      await memoryOpfs.readText(memoryOpfs.mediaPathFor('m1', 'h1', 'cache', 'data.json')),
    ).toContain('"v": 1')
  })

  it('reads as a miss after the source is replaced, and again after it is restored', async () => {
    memoryOpfs.setMedia('m1', 'h1')
    await writeJsonAtomic(CACHE_ROOT, MEDIA_FILE, { v: 1 })

    memoryOpfs.setMedia('m1', 'h2')
    expect(await readJson(CACHE_ROOT, MEDIA_FILE)).toBeNull()
    expect(await exists(CACHE_ROOT, MEDIA_FILE)).toBe(false)

    memoryOpfs.setMedia('m1', 'h1')
    expect(await readJson(CACHE_ROOT, MEDIA_FILE)).toEqual({ v: 1 })
  })

  it('writes with a pinned identity land under the pinned hash even after replacement', async () => {
    memoryOpfs.setMedia('m1', 'h1')
    const pinned = { workspaceId: memoryOpfs.workspaceId, mediaId: 'm1', contentHash: 'h1' }

    memoryOpfs.setMedia('m1', 'h2')
    await writeJsonAtomic(CACHE_ROOT, MEDIA_FILE, { v: 'old-bytes' }, pinned)

    expect(
      await memoryOpfs.readText(memoryOpfs.mediaPathFor('m1', 'h1', 'cache', 'data.json')),
    ).toContain('old-bytes')
    expect(await readJson(CACHE_ROOT, MEDIA_FILE)).toBeNull()
  })

  it('treats a media deleted on the server as a miss for every operation', async () => {
    memoryOpfs.setMedia('m1', 'h1')
    await writeJsonAtomic(CACHE_ROOT, MEDIA_FILE, { v: 1 })
    memoryOpfs.deleteMedia('m1')

    expect(await readJson(CACHE_ROOT, MEDIA_FILE)).toBeNull()
    expect(await readBlob(CACHE_ROOT, MEDIA_FILE)).toBeNull()
    expect(await exists(CACHE_ROOT, MEDIA_FILE)).toBe(false)
    expect(await listDirectory(CACHE_ROOT, ['media', 'm1', 'cache'])).toEqual([])
    expect(await readDirectoryFiles(CACHE_ROOT, ['media', 'm1', 'cache'])).toEqual([])
    await expect(removeEntry(CACHE_ROOT, MEDIA_FILE)).resolves.toBeUndefined()
    await expect(writeBlob(CACHE_ROOT, MEDIA_FILE, 'x')).resolves.toBeUndefined()
  })
})

describe('fs-primitives eviction and quota are misses', () => {
  it('drops a write that hits QuotaExceededError and keeps the previous file', async () => {
    await writeJsonAtomic(CACHE_ROOT, ['q.json'], { v: 1 })
    memoryOpfs.failWrites('QuotaExceededError')

    expect(await writeJsonAtomic(CACHE_ROOT, ['q.json'], { v: 2 })).toBe(0)
    await expect(writeBlob(CACHE_ROOT, ['q.bin'], new Uint8Array([1]))).resolves.toBeUndefined()

    memoryOpfs.failWrites(null)
    expect(await readJson(CACHE_ROOT, ['q.json'])).toEqual({ v: 1 })
    expect(await exists(CACHE_ROOT, ['q.bin'])).toBe(false)
  })

  it('still surfaces unexpected write failures', async () => {
    memoryOpfs.failWrites('SecurityError')
    await expect(writeJsonAtomic(CACHE_ROOT, ['q.json'], { v: 1 })).rejects.toThrow()
  })

  it('reads an entry evicted mid-read (NotReadableError) as a miss', async () => {
    await writeJsonAtomic(CACHE_ROOT, ['gone.json'], { v: 1 })
    memoryOpfs.failReads('NotReadableError')

    expect(await readJson(CACHE_ROOT, ['gone.json'])).toBeNull()
    expect(await readArrayBuffer(CACHE_ROOT, ['gone.json'])).toBeNull()
  })

  it('skips files that disappear between listing and reading', async () => {
    await writeBlob(CACHE_ROOT, ['dir', 'a.bin'], new Uint8Array([1]))
    await writeBlob(CACHE_ROOT, ['dir', 'b.bin'], new Uint8Array([2]))
    memoryOpfs.failReads('NotFoundError')

    expect(await readDirectoryFiles(CACHE_ROOT, ['dir'])).toEqual([])
  })
})

describe('fs-primitives enumeration and removal', () => {
  it('lists directories, filters readDirectoryFiles, and removes subtrees', async () => {
    await writeBlob(CACHE_ROOT, ['d', 'a.jpg'], new Uint8Array([1]))
    await writeBlob(CACHE_ROOT, ['d', 'b.txt'], new Uint8Array([2]))
    await writeBlob(CACHE_ROOT, ['d', 'sub', 'c.jpg'], new Uint8Array([3]))

    const entries = await listDirectory(CACHE_ROOT, ['d'])
    expect(entries.map((e) => `${e.kind}:${e.name}`).sort()).toEqual([
      'directory:sub',
      'file:a.jpg',
      'file:b.txt',
    ])

    const jpgs = await readDirectoryFiles(CACHE_ROOT, ['d'], (e) => e.name.endsWith('.jpg'))
    expect(jpgs.map((f) => f.name)).toEqual(['a.jpg'])

    await removeEntry(CACHE_ROOT, ['d'], { recursive: true })
    expect(await exists(CACHE_ROOT, ['d'])).toBe(false)
    await expect(removeEntry(CACHE_ROOT, ['d'], { recursive: true })).resolves.toBeUndefined()
  })

  it('removeFromEveryMedia clears one cache kind across media and content-hash versions', async () => {
    memoryOpfs.setMedia('m1', 'h1')
    memoryOpfs.setMedia('m2', 'h9')
    await writeBlob(CACHE_ROOT, ['media', 'm1', 'cache', 'filmstrip', '0.jpg'], 'a')
    await writeBlob(CACHE_ROOT, ['media', 'm1', 'cache', 'waveform', 'multi-res.bin'], 'w')
    await writeBlob(CACHE_ROOT, ['media', 'm2', 'cache', 'filmstrip', '0.jpg'], 'b')
    // A stale version of m1 that the server no longer references.
    memoryOpfs.setMedia('m1', 'h0')
    await writeBlob(CACHE_ROOT, ['media', 'm1', 'cache', 'filmstrip', '0.jpg'], 'c')
    memoryOpfs.setMedia('m1', 'h1')

    await removeFromEveryMedia(CACHE_ROOT, ['cache', 'filmstrip'])

    expect(
      await memoryOpfs.list(memoryOpfs.mediaPathFor('m1', 'h1', 'cache')),
    ).toEqual(['waveform'])
    expect(await memoryOpfs.list(memoryOpfs.mediaPathFor('m1', 'h0', 'cache'))).toEqual([])
    expect(await memoryOpfs.list(memoryOpfs.mediaPathFor('m2', 'h9', 'cache'))).toEqual([])
  })

  it('removeFromEveryMedia is a no-op when nothing is cached', async () => {
    await expect(removeFromEveryMedia(CACHE_ROOT, ['cache', 'filmstrip'])).resolves.toBeUndefined()
  })
})
