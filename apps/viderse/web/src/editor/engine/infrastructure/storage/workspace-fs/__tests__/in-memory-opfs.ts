/**
 * In-memory OPFS + fake `EditorStorage` for unit tests of the disposable cache
 * modules in workspace-fs.
 *
 * Usage in a test file:
 *
 *   vi.mock('@editor/storage/storage-backend', async () =>
 *     (await import('./__tests__/in-memory-opfs')).storageBackendMock,
 *   )
 *   import { memoryOpfs } from './__tests__/in-memory-opfs'
 *   beforeEach(() => memoryOpfs.reset())
 *
 * `reset()` installs a fresh OPFS tree as `navigator.storage.getDirectory()`
 * and clears the fake server's media list. `setMedia(id, contentHash)` is what
 * the fake server returns for `getMedia`/`listMedia`, so tests can replace a
 * source (new hash) or delete it (`deleteMedia`) exactly like the real server.
 *
 * Covers the OPFS subset the cache layer uses: getDirectoryHandle/getFileHandle
 * ({ create }), values(), removeEntry({ recursive }), file.getFile() (a real
 * Blob) and createWritable() with write/close/abort.
 */

import { vi } from 'vitest'

import { StorageClientError } from '@editor/storage/errors'

type Bytes = Uint8Array

class MemFile {
  readonly kind = 'file' as const
  constructor(
    public name: string,
    public data: Bytes = new Uint8Array(),
  ) {}
}

export class MemDir {
  readonly kind = 'directory' as const
  private readonly entries = new Map<string, MemDir | MemFile>()
  constructor(
    public name: string,
    private readonly ctl: OpfsControl,
  ) {}

  async getDirectoryHandle(name: string, options: { create?: boolean } = {}): Promise<MemDir> {
    const existing = this.entries.get(name)
    if (existing) {
      if (existing.kind !== 'directory') {
        throw new DOMException('Type mismatch', 'TypeMismatchError')
      }
      return existing
    }
    if (!options.create) {
      throw new DOMException('Not found', 'NotFoundError')
    }
    const dir = new MemDir(name, this.ctl)
    this.entries.set(name, dir)
    return dir
  }

  async getFileHandle(name: string, options: { create?: boolean } = {}): Promise<MemFileHandle> {
    const existing = this.entries.get(name)
    if (existing) {
      if (existing.kind !== 'file') {
        throw new DOMException('Type mismatch', 'TypeMismatchError')
      }
      return new MemFileHandle(existing, this.ctl)
    }
    if (!options.create) {
      throw new DOMException('Not found', 'NotFoundError')
    }
    const file = new MemFile(name)
    this.entries.set(name, file)
    return new MemFileHandle(file, this.ctl)
  }

  async removeEntry(name: string, options: { recursive?: boolean } = {}): Promise<void> {
    const existing = this.entries.get(name)
    if (!existing) {
      throw new DOMException('Not found', 'NotFoundError')
    }
    if (existing.kind === 'directory' && existing.entries.size > 0 && !options.recursive) {
      throw new DOMException('Directory not empty', 'InvalidModificationError')
    }
    this.entries.delete(name)
  }

  async *values(): AsyncIterableIterator<MemDir | MemFileHandle> {
    for (const entry of [...this.entries.values()]) {
      yield entry.kind === 'directory' ? entry : new MemFileHandle(entry, this.ctl)
    }
  }

  /** Test helper: names of this directory's direct children. */
  childNames(): string[] {
    return [...this.entries.keys()]
  }
}

export class MemFileHandle {
  readonly kind = 'file' as const

  constructor(
    private readonly file: MemFile,
    private readonly ctl: OpfsControl,
  ) {}

  get name(): string {
    return this.file.name
  }

  async createWritable(): Promise<MemWritable> {
    if (this.ctl.failWritesWith) {
      throw new DOMException('Simulated write failure', this.ctl.failWritesWith)
    }
    return new MemWritable(this.file)
  }

  async getFile(): Promise<File> {
    if (this.ctl.failReadsWith) {
      throw new DOMException('Simulated read failure', this.ctl.failReadsWith)
    }
    const bytes = this.file.data.slice()
    const file = new File([bytes], this.file.name)
    // jsdom omits these standard Blob APIs. Preserve real byte semantics for
    // cache consumers while retaining FileReader compatibility in this fixture.
    Object.defineProperties(file, {
      text: { value: async () => new TextDecoder().decode(bytes) },
      arrayBuffer: { value: async () => bytes.buffer },
    })
    return file
  }
}

export class MemWritable {
  private readonly chunks: Bytes[] = []

  constructor(private readonly file: MemFile) {}

  async write(data: unknown): Promise<void> {
    if (typeof data === 'string') {
      this.chunks.push(new TextEncoder().encode(data))
    } else if (data instanceof Uint8Array) {
      this.chunks.push(data)
    } else if (data instanceof ArrayBuffer) {
      this.chunks.push(new Uint8Array(data))
    } else if (data instanceof Blob) {
      this.chunks.push(new Uint8Array(await data.arrayBuffer()))
    } else {
      throw new TypeError('MemWritable.write: unsupported data type')
    }
  }

  /** Commit on close, like the real stream: an aborted write leaves the old file. */
  async close(): Promise<void> {
    const total = this.chunks.reduce((sum, c) => sum + c.byteLength, 0)
    const merged = new Uint8Array(total)
    let offset = 0
    for (const chunk of this.chunks) {
      merged.set(chunk, offset)
      offset += chunk.byteLength
    }
    this.file.data = merged
  }

  async abort(): Promise<void> {
    this.chunks.length = 0
  }
}

interface OpfsControl {
  failWritesWith: string | null
  failReadsWith: string | null
}

const WORKSPACE_ID = 'ws-test'

/** Directory-name segments the cache layer uses for a workspace + media version. */
function cachePrefix(): string[] {
  return ['ytf-editor-cache', WORKSPACE_ID]
}

class MemoryOpfs {
  private control: OpfsControl = { failWritesWith: null, failReadsWith: null }
  private mediaHashes = new Map<string, string>()
  root: MemDir = new MemDir('', this.control)
  workspaceId = WORKSPACE_ID

  /** Fresh empty OPFS tree and fake-server media list; installs the navigator stub. */
  reset(): void {
    this.control = { failWritesWith: null, failReadsWith: null }
    this.mediaHashes.clear()
    this.root = new MemDir('', this.control)
    vi.stubGlobal('navigator', {
      storage: { getDirectory: async () => this.root },
    })
  }

  /** Create or replace a media on the fake server (a new hash = replaced bytes). */
  setMedia(mediaId: string, contentHash: string): void {
    this.mediaHashes.set(mediaId, contentHash)
  }

  deleteMedia(mediaId: string): void {
    this.mediaHashes.delete(mediaId)
  }

  contentHash(mediaId: string): string {
    const hash = this.mediaHashes.get(mediaId)
    if (!hash) throw new StorageClientError('not_found', `Media ${mediaId} not found`, 404)
    return hash
  }

  listMedia(): Array<{ id: string; contentHash: string }> {
    return [...this.mediaHashes].map(([id, contentHash]) => ({ id, contentHash }))
  }

  /** Make every `createWritable()` reject with a DOMException of this name (e.g. QuotaExceededError). */
  failWrites(name: string | null): void {
    this.control.failWritesWith = name
  }

  /** Make every `getFile()` reject with a DOMException of this name. */
  failReads(name: string | null): void {
    this.control.failReadsWith = name
  }

  /** Absolute OPFS path for a non-media cache entry, e.g. `content/refs.json`. */
  pathFor(...segments: string[]): string[] {
    return [...cachePrefix(), ...segments.map(encodeURIComponent)]
  }

  /** Absolute OPFS path for a media cache entry under `contentHash`. */
  mediaPathFor(mediaId: string, contentHash: string, ...segments: string[]): string[] {
    return [
      ...cachePrefix(),
      'media',
      encodeURIComponent(mediaId),
      encodeURIComponent(contentHash),
      ...segments.map(encodeURIComponent),
    ]
  }

  async readText(path: string[]): Promise<string | null> {
    let dir = this.root
    try {
      for (const segment of path.slice(0, -1)) dir = await dir.getDirectoryHandle(segment)
      const handle = await dir.getFileHandle(path[path.length - 1]!)
      return await (await handle.getFile()).text()
    } catch {
      return null
    }
  }

  async plantText(path: string[], text: string): Promise<void> {
    let dir = this.root
    for (const segment of path.slice(0, -1)) dir = await dir.getDirectoryHandle(segment, { create: true })
    const handle = await dir.getFileHandle(path[path.length - 1]!, { create: true })
    const writable = await handle.createWritable()
    await writable.write(text)
    await writable.close()
  }

  async list(path: string[]): Promise<string[]> {
    let dir = this.root
    try {
      for (const segment of path) dir = await dir.getDirectoryHandle(segment)
    } catch {
      return []
    }
    return dir.childNames()
  }
}

export const memoryOpfs = new MemoryOpfs()

/** Module factory for `vi.mock('@editor/storage/storage-backend', ...)`. */
export const storageBackendMock = {
  getEditorStorage: () => ({
    workspaceId: async () => memoryOpfs.workspaceId,
    getMedia: async (id: string) => ({ id, contentHash: memoryOpfs.contentHash(id) }),
    listMedia: async () => memoryOpfs.listMedia(),
  }),
}
