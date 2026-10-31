import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { MediaMetadata } from '@editor/types/storage'
import type { MediaLibraryActions, MediaLibraryState } from '../types'
import { createImportActions } from './media-import-actions'
import { useMediaLibraryStore } from './media-library-store'
import { useMediaPreparationStore } from './media-preparation-store'

const mediaLibraryServiceMocks = vi.hoisted(() => ({
  importFile: vi.fn(),
  importMediaFromUrl: vi.fn(),
  importLottieFromUrl: vi.fn(),
  waitForMediaPreparation: vi.fn(),
  getMediaFile: vi.fn(),
}))

const mediaFilePickerMocks = vi.hoisted(() => ({
  pickMediaFiles: vi.fn(),
}))

const proxyServiceMocks = vi.hoisted(() => ({
  setProxyKey: vi.fn(),
  clearProxyKey: vi.fn(),
  canGenerateProxy: vi.fn(),
  hasProxy: vi.fn(),
  generateProxy: vi.fn(),
  loadExistingProxies: vi.fn(),
  onStatusChange: vi.fn(),
  setMediaResolver: vi.fn(),
  setFilmstripPrewarm: vi.fn(),
}))

const loggerMocks = vi.hoisted(() => ({
  startEvent: vi.fn(() => ({
    merge: vi.fn(),
    set: vi.fn(),
    success: vi.fn(),
    failure: vi.fn(),
  })),
  event: vi.fn(),
  error: vi.fn(),
  warn: vi.fn(),
  info: vi.fn(),
  debug: vi.fn(),
  child: vi.fn(),
  setLevel: vi.fn(),
}))

vi.mock('../services/media-library-service', () => ({
  mediaLibraryService: mediaLibraryServiceMocks,
}))

vi.mock('./media-library-service-access', () => ({
  loadMediaLibraryService: vi.fn(async () => ({
    mediaLibraryService: mediaLibraryServiceMocks,
  })),
}))

vi.mock('../utils/media-file-picker', () => ({
  pickMediaFiles: mediaFilePickerMocks.pickMediaFiles,
}))

vi.mock('../services/proxy-service', () => ({
  proxyService: proxyServiceMocks,
}))

vi.mock('../services/background-media-work', async () => {
  const { createBackgroundMediaWorkMocks } =
    await import('../test-utils/background-media-work-test-mocks')
  return createBackgroundMediaWorkMocks(vi)
})

vi.mock('@editor/infrastructure/storage', () => ({
  getTranscriptMediaIds: vi.fn(async () => new Set<string>()),
}))

vi.mock('./media-delete-actions', () => ({
  createDeleteActions: vi.fn(() => ({})),
}))

vi.mock('./media-relinking-actions', () => ({
  createRelinkingActions: vi.fn(() => ({})),
}))

vi.mock('../utils/validation', () => ({
  getMimeType: vi.fn((file: File) => file.type || 'application/octet-stream'),
  getMediaType: vi.fn((mimeType: string) => mimeType.split('/')[0]),
}))

vi.mock('../utils/proxy-key', () => ({
  getSharedProxyKey: vi.fn((media: { id: string }) => `proxy-${media.id}`),
}))

vi.mock('@editor/shared/logging/logger', () => ({
  createLogger: vi.fn(() => loggerMocks),
  createOperationId: vi.fn(() => 'op-test'),
}))

type ImportState = Partial<MediaLibraryState> & Partial<MediaLibraryActions>

function makeMedia(overrides: Partial<MediaMetadata> = {}): MediaMetadata {
  return {
    id: 'media-1',
    storageType: 'workspace',
    fileName: 'clip.mp4',
    fileSize: 1024,
    mimeType: 'video/mp4',
    duration: 4,
    width: 1920,
    height: 1080,
    fps: 30,
    codec: 'h264',
    bitrate: 5000,
    tags: [],
    createdAt: 1,
    updatedAt: 1,
    ...overrides,
  }
}

function mockDuplicateFileImport(fileName = 'clip.mp4') {
  const file = new File(['video'], fileName, { type: 'video/mp4' })
  const duplicate = makeMedia({ id: 'existing-1', fileName })
  const duplicateImport = {
    ...duplicate,
    isDuplicate: true,
  }

  mediaLibraryServiceMocks.importFile.mockResolvedValue(duplicateImport)

  return { file, duplicate, duplicateImport }
}

async function flushMicrotasks(times = 5): Promise<void> {
  for (let i = 0; i < times; i++) {
    await Promise.resolve()
  }
}

/**
 * Canonical `mediaItems` writes go through the command executor, which binds to
 * the real media-library store, so the harness drives that real store instead of
 * a synthetic set/get pair.
 */
function createImportActionsHarness(overrides: ImportState = {}) {
  useMediaLibraryStore.setState({
    currentProjectId: 'project-1',
    mediaItems: [],
    mediaById: {},
    isLoading: false,
    importingIds: [],
    error: null,
    errorLink: null,
    notification: null,
    selectedMediaIds: [],
    selectedCompositionIds: [],
    brokenMediaIds: [],
    unsupportedCodecFiles: [],
    showUnsupportedCodecDialog: false,
    unsupportedCodecResolver: null,
    showNotification: vi.fn(),
    ...overrides,
  })
  const actions = createImportActions(useMediaLibraryStore.setState, useMediaLibraryStore.getState)

  return {
    actions,
    get currentState() {
      return useMediaLibraryStore.getState()
    },
    setCurrentState(partial: Partial<MediaLibraryState>) {
      useMediaLibraryStore.setState(partial)
    },
  }
}

describe('createImportActions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    useMediaPreparationStore.getState().clearAll()
    mediaLibraryServiceMocks.importFile.mockReset()
    mediaLibraryServiceMocks.importMediaFromUrl.mockReset()
    mediaLibraryServiceMocks.importLottieFromUrl.mockReset()
    mediaLibraryServiceMocks.waitForMediaPreparation.mockReset()
    mediaLibraryServiceMocks.waitForMediaPreparation.mockResolvedValue(undefined)
    mediaLibraryServiceMocks.getMediaFile.mockReset()
    mediaFilePickerMocks.pickMediaFiles.mockReset()
    proxyServiceMocks.canGenerateProxy.mockImplementation((mimeType: string) =>
      mimeType.startsWith('video/'),
    )
    proxyServiceMocks.hasProxy.mockReturnValue(false)
  })

  it('replaces optimistic placeholders with imported media', async () => {
    const file = new File(['video'], 'clip.mp4', { type: 'video/mp4' })
    const imported = makeMedia({ id: 'imported-1', fileName: 'clip.mp4' })
    mediaLibraryServiceMocks.importFile.mockResolvedValue(imported)

    const harness = createImportActionsHarness()
    const { actions } = harness
    const result = await actions.importFiles([file])

    expect(result).toEqual([imported])
    expect(harness.currentState.mediaItems).toEqual([imported])
    expect(harness.currentState.importingIds).toEqual([])
    expect(proxyServiceMocks.setProxyKey).toHaveBeenCalledWith('imported-1', 'proxy-imported-1')
  })

  it('shows an ephemeral workspace placeholder while the server import is running', async () => {
    const file = new File(['video'], 'clip.mp4', { type: 'video/mp4' })
    const imported = makeMedia({ id: 'imported-1', fileName: 'clip.mp4' })
    let resolveImport!: (metadata: MediaMetadata) => void
    mediaLibraryServiceMocks.importFile.mockReturnValue(
      new Promise<MediaMetadata>((resolve) => {
        resolveImport = resolve
      }),
    )

    const harness = createImportActionsHarness({
      mediaItems: [makeMedia({ id: 'older-1', fileName: 'older.mp4' })],
    })
    const importPromise = harness.actions.importFiles([file])

    await flushMicrotasks()
    const placeholderId = harness.currentState.importingIds[0]
    expect(harness.currentState.mediaItems.map((item) => item.id)).toEqual([
      placeholderId,
      'older-1',
    ])
    expect(harness.currentState.mediaItems[0]).toEqual(
      expect.objectContaining({
        fileName: 'clip.mp4',
        storageType: 'workspace',
        codec: 'importing...',
      }),
    )

    resolveImport(imported)
    await importPromise

    expect(harness.currentState.mediaItems.map((item) => item.id)).toEqual([
      'imported-1',
      'older-1',
    ])
    expect(harness.currentState.importingIds).toEqual([])
  })

  it('tracks optimistic imports in the unified preparation queue', async () => {
    const file = new File(['video'], 'clip.mp4', { type: 'video/mp4' })
    const imported = makeMedia({ id: 'imported-1', fileName: 'clip.mp4' })
    let resolveImport!: (metadata: MediaMetadata) => void
    mediaLibraryServiceMocks.importFile.mockReturnValue(
      new Promise<MediaMetadata>((resolve) => {
        resolveImport = resolve
      }),
    )

    const harness = createImportActionsHarness()
    const importPromise = harness.actions.importFiles([file])

    await flushMicrotasks()
    const placeholderId = harness.currentState.importingIds[0]
    expect([...useMediaPreparationStore.getState().tasks.values()]).toEqual([
      expect.objectContaining({
        mediaId: placeholderId,
        type: 'import',
        status: 'running',
      }),
    ])

    resolveImport(imported)
    await importPromise

    expect([...useMediaPreparationStore.getState().tasks.values()]).toEqual([])
  })

  it('processes dropped files with bounded parallelism while preserving result order', async () => {
    const firstFile = new File(['video-1'], 'first.mp4', { type: 'video/mp4' })
    const secondFile = new File(['video-2'], 'second.mp4', { type: 'video/mp4' })
    const files = [firstFile, secondFile]
    const imported = [
      makeMedia({ id: 'first-imported', fileName: 'first.mp4' }),
      makeMedia({ id: 'second-imported', fileName: 'second.mp4' }),
    ]
    let firstResolve!: (metadata: MediaMetadata) => void
    const firstImport = new Promise<MediaMetadata>((resolve) => {
      firstResolve = resolve
    })

    mediaLibraryServiceMocks.importFile
      .mockReturnValueOnce(firstImport)
      .mockResolvedValueOnce(imported[1])

    const harness = createImportActionsHarness()
    const importPromise = harness.actions.importFiles(files)

    await flushMicrotasks()
    expect(mediaLibraryServiceMocks.importFile).toHaveBeenCalledTimes(2)

    firstResolve(imported[0]!)
    const result = await importPromise

    expect(result.map((item) => item.id)).toEqual(['first-imported', 'second-imported'])
    expect(mediaLibraryServiceMocks.importFile).toHaveBeenNthCalledWith(1, firstFile, {
      projectId: 'project-1',
    })
    expect(mediaLibraryServiceMocks.importFile).toHaveBeenNthCalledWith(2, secondFile, {
      projectId: 'project-1',
    })
  })

  it('does not start more than two server imports at once', async () => {
    const files = [
      new File(['1'], 'one.mp4', { type: 'video/mp4' }),
      new File(['2'], 'two.mp4', { type: 'video/mp4' }),
      new File(['3'], 'three.mp4', { type: 'video/mp4' }),
    ]
    const resolvers: Array<(metadata: MediaMetadata) => void> = []
    mediaLibraryServiceMocks.importFile.mockImplementation(
      () =>
        new Promise<MediaMetadata>((resolve) => {
          resolvers.push(resolve)
        }),
    )

    const harness = createImportActionsHarness()
    const importPromise = harness.actions.importFiles(files)

    await flushMicrotasks()
    expect(mediaLibraryServiceMocks.importFile).toHaveBeenCalledTimes(2)

    resolvers[0]!(makeMedia({ id: 'one-imported', fileName: 'one.mp4' }))
    await flushMicrotasks()
    expect(mediaLibraryServiceMocks.importFile).toHaveBeenCalledTimes(3)

    resolvers[1]!(makeMedia({ id: 'two-imported', fileName: 'two.mp4' }))
    resolvers[2]!(makeMedia({ id: 'three-imported', fileName: 'three.mp4' }))
    const result = await importPromise

    expect(result.map((item) => item.id)).toEqual(['one-imported', 'two-imported', 'three-imported'])
  })

  it('still lands imported media when a concurrent reload wipes the placeholder', async () => {
    // Regression: removing all media then re-importing can trigger a
    // `loadMediaItems()` that replaces `mediaItems` from disk mid-import,
    // wiping the optimistic placeholder. A `.map`-only swap would match
    // nothing and silently drop the import (vanishes until full reload).
    const file = new File(['video'], 'clip.mp4', { type: 'video/mp4' })
    const imported = makeMedia({ id: 'imported-1', fileName: 'clip.mp4' })
    const harness = createImportActionsHarness()

    // Simulate the concurrent reload landing while the import is in flight.
    mediaLibraryServiceMocks.importFile.mockImplementation(async () => {
      harness.setCurrentState({
        mediaItems: [],
        importingIds: [],
      })
      return imported
    })

    const result = await harness.actions.importFiles([file])

    expect(result).toEqual([imported])
    expect(harness.currentState.mediaItems).toEqual([imported])
    expect(harness.currentState.importingIds).toEqual([])
  })

  it('does not duplicate when the reload already re-added the imported media', async () => {
    // If the concurrent reload re-reads the freshly-associated media from
    // disk before the placeholder swap runs, the swap must refresh in place
    // rather than prepend a second copy.
    const file = new File(['video'], 'clip.mp4', { type: 'video/mp4' })
    const imported = makeMedia({ id: 'imported-1', fileName: 'clip.mp4' })

    const harness = createImportActionsHarness()

    mediaLibraryServiceMocks.importFile.mockImplementation(async () => {
      harness.setCurrentState({
        mediaItems: [imported],
        importingIds: [],
      })
      return imported
    })

    const result = await harness.actions.importFiles([file])

    expect(result).toEqual([imported])
    expect(harness.currentState.mediaItems).toEqual([imported])
    expect(harness.currentState.mediaItems.filter((m) => m.id === 'imported-1')).toHaveLength(1)
  })

  it('stores only the canonical record and reports unsupported audio codecs', async () => {
    const file = new File(['video'], 'clip.mp4', { type: 'video/mp4' })
    const imported = makeMedia({ id: 'imported-1', fileName: 'clip.mp4', audioCodec: 'ac3' })
    mediaLibraryServiceMocks.importFile.mockResolvedValue({
      ...imported,
      hasUnsupportedCodec: true,
    })

    const harness = createImportActionsHarness()
    const result = await harness.actions.importFiles([file])

    expect(result).toEqual([{ ...imported, hasUnsupportedCodec: true }])
    expect(harness.currentState.mediaItems).toEqual([imported])
    expect(harness.currentState.mediaItems[0]).not.toHaveProperty('hasUnsupportedCodec')
    expect(harness.currentState.showNotification).toHaveBeenCalledWith({
      type: 'warning',
      message:
        'Đã nhập 1 tệp. 1 tệp có codec âm thanh không được hỗ trợ (ac3). Có thể không có dạng sóng âm thanh.',
    })
  })

  it('imports media from a direct URL and prepends it to the library', async () => {
    const imported = makeMedia({ id: 'remote-1', fileName: 'clip.mp4' })
    mediaLibraryServiceMocks.importMediaFromUrl.mockResolvedValue(imported)

    const harness = createImportActionsHarness({
      mediaItems: [makeMedia({ id: 'older-1', fileName: 'older.mp4' })],
    })

    const result = await harness.actions.importMediaFromUrl('https://cdn.example.com/clip.mp4')

    expect(result).toEqual([imported])
    expect(mediaLibraryServiceMocks.importMediaFromUrl).toHaveBeenCalledWith(
      'https://cdn.example.com/clip.mp4',
      'project-1',
    )
    expect(harness.currentState.mediaItems.map((item) => item.id)).toEqual(['remote-1', 'older-1'])
    expect(proxyServiceMocks.setProxyKey).toHaveBeenCalledWith('remote-1', 'proxy-remote-1')
  })

  it('shows an info notification when a URL import resolves to an existing media item', async () => {
    const duplicate = makeMedia({ id: 'existing-1', fileName: 'clip.mp4' })
    mediaLibraryServiceMocks.importMediaFromUrl.mockResolvedValue({
      ...duplicate,
      isDuplicate: true,
    })

    const harness = createImportActionsHarness()
    const result = await harness.actions.importMediaFromUrl('https://cdn.example.com/clip.mp4')

    expect(result).toEqual([])
    expect(harness.currentState.mediaItems).toEqual([])
    expect(harness.currentState.showNotification).toHaveBeenCalledWith({
      type: 'info',
      message: '"clip.mp4" đã có trong thư viện',
    })
    expect(proxyServiceMocks.setProxyKey).not.toHaveBeenCalled()
  })

  it('imports a remote Lottie animation with attribution and prepends it to the library', async () => {
    const attribution = { provider: 'lottiefiles' } as const
    const imported = makeMedia({
      id: 'lottie-1',
      fileName: 'Hello.lottie',
      mimeType: 'application/lottie+json',
    })
    mediaLibraryServiceMocks.importLottieFromUrl.mockResolvedValue(imported)

    const harness = createImportActionsHarness({
      mediaItems: [makeMedia({ id: 'older-1', fileName: 'older.mp4' })],
    })

    const result = await harness.actions.importRemoteLottie({
      url: 'https://cdn.example.com/hello.lottie',
      fileName: 'Hello.lottie',
      attribution,
    })

    expect(result).toEqual(imported)
    expect(mediaLibraryServiceMocks.importLottieFromUrl).toHaveBeenCalledWith(
      'https://cdn.example.com/hello.lottie',
      'project-1',
      { fileName: 'Hello.lottie', attribution },
    )
    expect(harness.currentState.mediaItems.map((item) => item.id)).toEqual(['lottie-1', 'older-1'])
  })

  it('re-adds a removed-but-associated file with no "already exists" banner', async () => {
    // Regression: re-importing a file you removed from the library view (but
    // whose project association lingered / was re-backfilled) is the
    // by-design cross-workspace dedup re-associating it. That is a normal
    // (re-)add — it must surface in the library WITHOUT the "already exists"
    // banner, not be stranded behind it.
    const { file, duplicate, duplicateImport } = mockDuplicateFileImport()

    const harness = createImportActionsHarness() // library empty — file not visible here
    const result = await harness.actions.importFiles([file])

    expect(result).toEqual([duplicateImport])
    expect(harness.currentState.mediaItems).toEqual([duplicate])
    expect(harness.currentState.importingIds).toEqual([])
    expect(harness.currentState.showNotification).not.toHaveBeenCalled()
    expect(proxyServiceMocks.setProxyKey).toHaveBeenCalledWith('existing-1', 'proxy-existing-1')
  })

  it('shows "already exists" only for a file already visible in the library', async () => {
    const { file, duplicate } = mockDuplicateFileImport()

    const harness = createImportActionsHarness({ mediaItems: [duplicate] })

    const result = await harness.actions.importFiles([file])

    expect(result).toEqual([])
    expect(harness.currentState.mediaItems).toHaveLength(1)
    expect(harness.currentState.mediaItems[0]?.id).toBe('existing-1')
    expect(harness.currentState.importingIds).toEqual([])
    expect(harness.currentState.showNotification).toHaveBeenCalledWith({
      type: 'info',
      message: '"clip.mp4" đã có trong thư viện',
    })
    expect(proxyServiceMocks.setProxyKey).not.toHaveBeenCalled()
  })

  it('returns duplicates for placement flows and surfaces them in the library', async () => {
    const { file, duplicate, duplicateImport } = mockDuplicateFileImport()

    const harness = createImportActionsHarness()
    const { actions } = harness
    const result = await actions.importFilesForPlacement([file])

    expect(result).toEqual([duplicateImport])
    expect(harness.currentState.mediaItems).toEqual([duplicate])
    expect(harness.currentState.importingIds).toEqual([])
    expect(mediaLibraryServiceMocks.waitForMediaPreparation).toHaveBeenCalledWith(['existing-1'])
  })

  it('waits for media preparation before resolving placement imports', async () => {
    const file = new File(['video'], 'clip.mp4', { type: 'video/mp4' })
    const imported = makeMedia({ id: 'imported-1', fileName: 'clip.mp4' })
    let resolvePreparation!: () => void
    mediaLibraryServiceMocks.importFile.mockResolvedValue(imported)
    mediaLibraryServiceMocks.waitForMediaPreparation.mockReturnValue(
      new Promise<void>((resolve) => {
        resolvePreparation = resolve
      }),
    )

    const harness = createImportActionsHarness()
    const resultPromise = harness.actions.importFilesForPlacement([file])

    await flushMicrotasks()
    let didResolve = false
    resultPromise.then(() => {
      didResolve = true
    })
    await flushMicrotasks()

    expect(didResolve).toBe(false)
    expect(mediaLibraryServiceMocks.waitForMediaPreparation).toHaveBeenCalledWith(['imported-1'])

    resolvePreparation()
    await expect(resultPromise).resolves.toEqual([imported])
  })

  it('cleans up failed placeholders and reports import errors', async () => {
    const file = new File(['video'], 'clip.mp4', { type: 'video/mp4' })
    mediaLibraryServiceMocks.importFile.mockRejectedValue(new Error('Import failed'))

    const harness = createImportActionsHarness()
    const result = await harness.actions.importFiles([file])

    expect(result).toEqual([])
    expect(harness.currentState.mediaItems).toEqual([])
    expect(harness.currentState.importingIds).toEqual([])
    expect(loggerMocks.error).toHaveBeenCalledWith('Failed to import clip.mp4', expect.any(Error))
    expect(harness.currentState.showNotification).toHaveBeenCalledWith({
      type: 'warning',
      message: '1 tệp nhập thất bại. Hãy kiểm tra tệp rồi thử lại.',
    })
  })

  it('reports mixed import outcomes in one summary instead of overwriting earlier results', async () => {
    const importedFile = new File(['video'], 'imported.mp4', { type: 'video/mp4' })
    const duplicateFile = new File(['video'], 'duplicate.mp4', { type: 'video/mp4' })
    const failedFile = new File(['video'], 'failed.mp4', { type: 'video/mp4' })
    const files = [importedFile, duplicateFile, failedFile]
    const imported = makeMedia({ id: 'imported-1', fileName: 'imported.mp4' })
    const duplicate = makeMedia({ id: 'duplicate-1', fileName: 'duplicate.mp4' })

    mediaLibraryServiceMocks.importFile
      .mockResolvedValueOnce(imported)
      .mockResolvedValueOnce({ ...duplicate, isDuplicate: true })
      .mockRejectedValueOnce(new Error('Import failed'))

    const harness = createImportActionsHarness({ mediaItems: [duplicate] })

    const result = await harness.actions.importFiles(files)

    expect(result).toEqual([imported])
    expect(harness.currentState.showNotification).toHaveBeenCalledTimes(1)
    expect(harness.currentState.showNotification).toHaveBeenCalledWith({
      type: 'warning',
      message:
        'Đã nhập 1 tệp. Đã bỏ qua 1 tệp trùng: duplicate.mp4. 1 tệp nhập thất bại. Hãy kiểm tra tệp rồi thử lại.',
    })
  })

  it('reports an error without importing when no project is selected', async () => {
    const harness = createImportActionsHarness({ currentProjectId: null })

    const result = await harness.actions.importFiles([
      new File(['video'], 'clip.mp4', { type: 'video/mp4' }),
    ])

    expect(result).toEqual([])
    expect(harness.currentState.error).toBe('Chưa chọn dự án')
    expect(harness.currentState.mediaItems).toEqual([])
    expect(mediaLibraryServiceMocks.importFile).not.toHaveBeenCalled()
  })

  it('imports files chosen in the picker through the server import', async () => {
    const file = new File(['video'], 'clip.mp4', { type: 'video/mp4' })
    const imported = makeMedia({ id: 'imported-1', fileName: 'clip.mp4' })
    mediaFilePickerMocks.pickMediaFiles.mockResolvedValue([file])
    mediaLibraryServiceMocks.importFile.mockResolvedValue(imported)

    const harness = createImportActionsHarness()
    const result = await harness.actions.importMedia()

    expect(mediaFilePickerMocks.pickMediaFiles).toHaveBeenCalledWith({ multiple: true })
    expect(mediaLibraryServiceMocks.importFile).toHaveBeenCalledWith(file, {
      projectId: 'project-1',
    })
    expect(result).toEqual([imported])
    expect(harness.currentState.mediaItems).toEqual([imported])
    expect(harness.currentState.error).toBeNull()
  })

  it('treats a cancelled picker as a silent no-op', async () => {
    mediaFilePickerMocks.pickMediaFiles.mockRejectedValue(
      new DOMException('The user aborted a request.', 'AbortError'),
    )

    const harness = createImportActionsHarness()
    const result = await harness.actions.importMedia()

    expect(result).toEqual([])
    expect(harness.currentState.error).toBeNull()
    expect(harness.currentState.mediaItems).toEqual([])
    expect(mediaLibraryServiceMocks.importFile).not.toHaveBeenCalled()
  })

  it('surfaces picker failures as a library error', async () => {
    mediaFilePickerMocks.pickMediaFiles.mockRejectedValue(new Error('Picker exploded'))

    const harness = createImportActionsHarness()
    const result = await harness.actions.importMedia()

    expect(result).toEqual([])
    expect(harness.currentState.error).toBe('Picker exploded')
  })
})
