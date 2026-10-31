import type { MediaLibraryState, MediaLibraryActions, UnsupportedCodecFile } from '../types'
import type { MediaAttribution, MediaMetadata } from '@editor/types/storage'
import type { mediaLibraryService } from '../services/media-library-service'
import { executeCommands } from '@editor/commands'
import { loadMediaLibraryService } from './media-library-service-access'
import { proxyService } from '../services/proxy-service'
import { getMimeType } from '../utils/validation'
import { getSharedProxyKey } from '../utils/proxy-key'
import { pickMediaFiles } from '../utils/media-file-picker'
import { createLogger, createOperationId } from '@editor/shared/logging/logger'
import { useMediaPreparationStore } from './media-preparation-store'

const logger = createLogger('MediaImport')

type Set = (
  partial:
    | Partial<MediaLibraryState>
    | ((state: MediaLibraryState & MediaLibraryActions) => Partial<MediaLibraryState>),
) => void
type Get = () => MediaLibraryState & MediaLibraryActions

type ImportedMetadata = MediaMetadata & { isDuplicate?: boolean; hasUnsupportedCodec?: boolean }

const IMPORT_PROCESSING_CONCURRENCY = 2

interface ImportTask {
  tempId: string
  file: File
}

type MediaLibraryService = typeof mediaLibraryService

interface CompletedImportTask extends ImportTask {
  metadata: ImportedMetadata
}

function buildOptimisticMediaItem(file: File, tempId: string): MediaMetadata {
  const now = Date.now()

  return {
    id: tempId,
    storageType: 'workspace',
    fileName: file.name,
    fileSize: file.size,
    fileLastModified: file.lastModified,
    mimeType: getMimeType(file),
    duration: 0,
    width: 0,
    height: 0,
    fps: 30,
    codec: 'importing...',
    bitrate: 0,
    tags: [],
    createdAt: now,
    updatedAt: now,
  }
}

/**
 * Strip the transient import flags so only the canonical media record is
 * written into the library.
 */
function toMediaRecord(metadata: ImportedMetadata): MediaMetadata {
  const record: ImportedMetadata = { ...metadata }
  delete record.isDuplicate
  delete record.hasUnsupportedCodec
  return record
}

/** Canonical media mutation: always goes through the command executor. */
function addMediaRecord(metadata: ImportedMetadata): void {
  const result = executeCommands([{ type: 'media.add', params: { media: toMediaRecord(metadata) } }])
  if (!result.ok) {
    throw new Error(result.issues.map((issue) => issue.message).join('; ') || result.error)
  }
}

/** Optimistic placeholders are ephemeral UI state and never enter command history. */
function removeImportPlaceholder(set: Set, tempId: string): void {
  useMediaPreparationStore.getState().clearMedia(tempId)

  set((state) => ({
    mediaItems: state.mediaItems.filter((item) => item.id !== tempId),
    importingIds: state.importingIds.filter((id) => id !== tempId),
  }))
}

/**
 * Drop the optimistic placeholder and guarantee the resolved media record is
 * visible in the library exactly once.
 *
 * Used for BOTH fresh imports and "duplicate" results. A re-imported file is
 * flagged `isDuplicate` whenever it already has a project association — but
 * that association can outlive the file's presence in the in-memory library
 * (the user removed it from the library view, or the association was
 * re-backfilled from a lingering timeline clip). The old duplicate path only
 * removed the placeholder, so re-importing such a file showed "already exists
 * in library" while the file stayed invisible and un-recoverable without a
 * full reload. Surfacing it here fixes that.
 *
 * Also resilient to a concurrent `loadMediaItems()` wiping the placeholder
 * mid-import: the record is upserted by the `media.add` command rather than
 * silently dropped.
 */
function ensureImportedMediaVisible(
  set: Set,
  get: Get,
  tempId: string,
  metadata: ImportedMetadata,
): boolean {
  removeImportPlaceholder(set, tempId)
  const wasAlreadyVisible = get().mediaItems.some((item) => item.id === metadata.id)
  addMediaRecord(metadata)
  return wasAlreadyVisible
}

function prependImportedMedia(set: Set, metadata: ImportedMetadata): void {
  addMediaRecord(metadata)
  set({ error: null, errorLink: null })
}

function setupImportedVideoProxy(metadata: MediaMetadata): void {
  if (!proxyService.canGenerateProxy(metadata.mimeType)) {
    return
  }

  // ProRes and other browser-undecodable codecs are previewed via live turbores
  // decode (full fidelity), not an auto-generated proxy. A proxy remains an opt-in
  // scrub-performance aid via the media library UI, so we still map its key here.
  proxyService.setProxyKey(metadata.id, getSharedProxyKey(metadata))
}

function queueImportPreparationTask(tempId: string): void {
  const preparationStore = useMediaPreparationStore.getState()
  preparationStore.queueTask(tempId, 'import')
  preparationStore.updateTask(tempId, 'import', { status: 'queued', progress: 0.05 })
}

function markImportPreparationRunning(tempId: string): void {
  useMediaPreparationStore
    .getState()
    .updateTask(tempId, 'import', { status: 'running', progress: 0.2 })
}

function processImportResults(
  importResults: PromiseSettledResult<CompletedImportTask>[],
  importTasks: ImportTask[],
  set: Set,
  get: Get,
  options?: { includeDuplicatesInResults?: boolean },
): {
  results: MediaMetadata[]
  importedCount: number
  duplicateNames: string[]
  unsupportedCodecFiles: UnsupportedCodecFile[]
  failedCount: number
} {
  const results: MediaMetadata[] = []
  const duplicateNames: string[] = []
  const unsupportedCodecFiles: UnsupportedCodecFile[] = []
  let importedCount = 0
  let failedCount = 0

  importResults.forEach((result, index) => {
    const importTask = importTasks[index]
    if (!importTask) {
      return
    }

    if (result.status === 'fulfilled') {
      const { metadata, tempId, file } = result.value

      const wasAlreadyVisible = ensureImportedMediaVisible(set, get, tempId, metadata)

      // "already exists in library" should only fire for a genuine no-op:
      // re-importing a file that is ALREADY visible in this project's library.
      // A file flagged `isDuplicate` merely has a project↔media association —
      // which the by-design cross-workspace dedup re-creates when you re-import
      // a file you'd removed. Surfacing that as "already exists" is wrong; it's
      // a normal (re-)add, so fall through to the import branch with no banner.
      if (metadata.isDuplicate && wasAlreadyVisible) {
        duplicateNames.push(file.name)
        if (options?.includeDuplicatesInResults) {
          results.push(metadata)
        }
      } else {
        setupImportedVideoProxy(metadata)
        results.push(metadata)
        importedCount += 1

        if (metadata.hasUnsupportedCodec && metadata.audioCodec) {
          unsupportedCodecFiles.push({
            fileName: file.name,
            audioCodec: metadata.audioCodec,
          })
        }
      }
    } else {
      failedCount++
      removeImportPlaceholder(set, importTask.tempId)
      logger.error(`Failed to import ${importTask.file.name}`, result.reason)
    }
  })

  return { results, importedCount, duplicateNames, unsupportedCodecFiles, failedCount }
}

function formatNameList(names: string[]): string {
  if (names.length === 0) return ''
  if (names.length <= 3) return names.join(', ')
  return `${names.slice(0, 3).join(', ')} và ${names.length - 3} tệp khác`
}

function buildImportSummaryMessage({
  importedCount,
  duplicateNames,
  unsupportedCodecFiles,
  failedCount,
}: {
  importedCount: number
  duplicateNames: string[]
  unsupportedCodecFiles: UnsupportedCodecFile[]
  failedCount: number
}): string | null {
  const hasProblems =
    duplicateNames.length > 0 || unsupportedCodecFiles.length > 0 || failedCount > 0
  if (!hasProblems) return null

  const parts: string[] = []

  // Keep clean imports quiet, but include the successful count when the user
  // also needs to know what was skipped or failed.
  if (importedCount > 0) {
    parts.push(`Đã nhập ${importedCount} tệp.`)
  }

  if (duplicateNames.length > 0) {
    if (importedCount === 0 && unsupportedCodecFiles.length === 0 && failedCount === 0) {
      parts.push(
        duplicateNames.length === 1
          ? `"${duplicateNames[0]}" đã có trong thư viện`
          : `${duplicateNames.length} tệp đã có trong thư viện`,
      )
    } else if (duplicateNames.length === 1) {
      parts.push(`Đã bỏ qua 1 tệp trùng: ${duplicateNames[0]}.`)
    } else {
      parts.push(`Đã bỏ qua ${duplicateNames.length} tệp trùng: ${formatNameList(duplicateNames)}.`)
    }
  }

  if (unsupportedCodecFiles.length > 0) {
    const codecList = [...new Set(unsupportedCodecFiles.map((f) => f.audioCodec))].join(', ')
    parts.push(
      `${unsupportedCodecFiles.length} tệp có codec âm thanh không được hỗ trợ (${codecList}). Có thể không có dạng sóng âm thanh.`,
    )
  }

  if (failedCount > 0) {
    parts.push(
      failedCount === 1
        ? '1 tệp nhập thất bại. Hãy kiểm tra tệp rồi thử lại.'
        : `${failedCount} tệp nhập thất bại. Hãy kiểm tra các tệp rồi thử lại.`,
    )
  }

  return parts.length > 0 ? parts.join(' ') : null
}

function showImportNotifications(
  importedCount: number,
  duplicateNames: string[],
  unsupportedCodecFiles: UnsupportedCodecFile[],
  failedCount: number,
  get: Get,
): void {
  const message = buildImportSummaryMessage({
    importedCount,
    duplicateNames,
    unsupportedCodecFiles,
    failedCount,
  })
  if (!message) return

  const type = failedCount > 0 || unsupportedCodecFiles.length > 0 ? 'warning' : 'info'
  get().showNotification({ type, message })
}

export function createImportActions(
  set: Set,
  get: Get,
): Pick<
  MediaLibraryActions,
  | 'importMedia'
  | 'importMediaFromUrl'
  | 'importRemoteLottie'
  | 'importFiles'
  | 'importFilesForPlacement'
> {
  const createOptimisticImportTasks = (files: File[]): ImportTask[] => {
    const importTasks: ImportTask[] = []

    for (const file of files) {
      const tempId = crypto.randomUUID()
      const tempItem = buildOptimisticMediaItem(file, tempId)

      set((state) => ({
        mediaItems: [tempItem, ...state.mediaItems],
        importingIds: [...state.importingIds, tempId],
        error: null,
      }))
      queueImportPreparationTask(tempId)

      importTasks.push({ tempId, file })
    }

    return importTasks
  }

  const runImportTasks = async (
    importTasks: ImportTask[],
    projectId: string,
    serviceModulePromise: Promise<{ mediaLibraryService: MediaLibraryService }>,
  ): Promise<PromiseSettledResult<CompletedImportTask>[]> => {
    const results: PromiseSettledResult<CompletedImportTask>[] = new Array(importTasks.length)
    let nextIndex = 0
    const { mediaLibraryService } = await serviceModulePromise

    const runNext = async (): Promise<void> => {
      while (nextIndex < importTasks.length) {
        const index = nextIndex++
        const task = importTasks[index]
        if (!task) {
          continue
        }

        try {
          markImportPreparationRunning(task.tempId)
          const metadata = await mediaLibraryService.importFile(task.file, { projectId })
          results[index] = {
            status: 'fulfilled',
            value: { metadata, tempId: task.tempId, file: task.file },
          }
        } catch (reason) {
          results[index] = { status: 'rejected', reason }
        }
      }
    }

    const workerCount = Math.min(IMPORT_PROCESSING_CONCURRENCY, importTasks.length)
    await Promise.all(Array.from({ length: workerCount }, runNext))
    return results
  }

  const importFilesInternal = async (
    files: File[],
    options?: {
      includeDuplicatesInResults?: boolean
      waitForPreparation?: boolean
    },
  ): Promise<MediaMetadata[]> => {
    const { currentProjectId } = get()

    if (!currentProjectId) {
      set({ error: 'Chưa chọn dự án' })
      return []
    }

    const opId = createOperationId()
    const event = logger.startEvent('import', opId)
    event.merge({
      source: 'drag-drop',
      projectId: currentProjectId,
      fileCount: files.length,
    })

    const serviceModulePromise = loadMediaLibraryService()
    const importTasks = createOptimisticImportTasks(files)
    const importResults = await runImportTasks(importTasks, currentProjectId, serviceModulePromise)

    const { results, importedCount, duplicateNames, unsupportedCodecFiles, failedCount } =
      processImportResults(importResults, importTasks, set, get, options)

    showImportNotifications(importedCount, duplicateNames, unsupportedCodecFiles, failedCount, get)

    if (options?.waitForPreparation && results.length > 0) {
      const { mediaLibraryService } = await serviceModulePromise
      await mediaLibraryService.waitForMediaPreparation(results.map((media) => media.id))
    }

    event.success({
      imported: importedCount,
      duplicates: duplicateNames.length,
      failed: failedCount,
      unsupportedCodecs: unsupportedCodecFiles.length,
    })

    return results
  }

  return {
    importMedia: async () => {
      const { currentProjectId } = get()

      if (!currentProjectId) {
        set({ error: 'Chưa chọn dự án' })
        return []
      }

      const opId = createOperationId()
      const event = logger.startEvent('import', opId)
      event.set('source', 'picker')
      event.set('projectId', currentProjectId)

      try {
        // Open file picker
        const files = await pickMediaFiles({ multiple: true })

        event.set('fileCount', files.length)

        // Create optimistic placeholders for all files immediately
        const serviceModulePromise = loadMediaLibraryService()
        const importTasks = createOptimisticImportTasks(files)
        const importResults = await runImportTasks(
          importTasks,
          currentProjectId,
          serviceModulePromise,
        )

        const { results, importedCount, duplicateNames, unsupportedCodecFiles, failedCount } =
          processImportResults(importResults, importTasks, set, get)

        showImportNotifications(
          importedCount,
          duplicateNames,
          unsupportedCodecFiles,
          failedCount,
          get,
        )

        event.success({
          imported: importedCount,
          duplicates: duplicateNames.length,
          failed: failedCount,
          unsupportedCodecs: unsupportedCodecFiles.length,
        })

        return results
      } catch (error) {
        // User cancelled or error
        if (error instanceof Error && error.name !== 'AbortError') {
          set({ error: error.message })
          event.failure(error)
        } else {
          event.success({
            outcome: 'cancelled',
            imported: 0,
            duplicates: 0,
            failed: 0,
            unsupportedCodecs: 0,
          })
        }
        return []
      }
    },

    importMediaFromUrl: async (url: string) => {
      const { currentProjectId } = get()
      const trimmedUrl = url.trim()

      if (!currentProjectId) {
        set({ error: 'Chưa chọn dự án', errorLink: null })
        return []
      }

      if (trimmedUrl.length === 0) {
        set({ error: 'Hãy nhập URL media.', errorLink: null })
        return []
      }

      set({ error: null, errorLink: null })

      const opId = createOperationId()
      const event = logger.startEvent('import', opId)
      event.set('source', 'url')
      event.set('projectId', currentProjectId)

      try {
        const parsedUrl = new URL(trimmedUrl)
        event.set('urlHost', parsedUrl.hostname)
      } catch {
        event.set('urlHost', 'invalid')
      }

      try {
        const { mediaLibraryService } = await loadMediaLibraryService()
        const metadata = await mediaLibraryService.importMediaFromUrl(trimmedUrl, currentProjectId)

        if (metadata.isDuplicate) {
          showImportNotifications(0, [metadata.fileName], [], 0, get)
          event.success({
            imported: 0,
            duplicates: 1,
            failed: 0,
            unsupportedCodecs: 0,
          })
          return []
        }

        prependImportedMedia(set, metadata)
        setupImportedVideoProxy(metadata)

        const unsupportedCodecFiles =
          metadata.hasUnsupportedCodec && metadata.audioCodec
            ? [{ fileName: metadata.fileName, audioCodec: metadata.audioCodec }]
            : []
        showImportNotifications(1, [], unsupportedCodecFiles, 0, get)

        event.success({
          imported: 1,
          duplicates: 0,
          failed: 0,
          unsupportedCodecs: unsupportedCodecFiles.length,
        })
        return [metadata]
      } catch (error) {
        const importError = error instanceof Error ? error : new Error(String(error))
        set({ error: importError.message, errorLink: null })
        event.failure(importError)
        return []
      }
    },

    importRemoteLottie: async (params: {
      url: string
      fileName?: string
      attribution?: MediaAttribution
    }) => {
      const { currentProjectId } = get()

      if (!currentProjectId) {
        set({ error: 'Chưa chọn dự án', errorLink: null })
        return null
      }

      set({ error: null, errorLink: null })

      const opId = createOperationId()
      const event = logger.startEvent('import', opId)
      event.set('source', 'lottiefiles')
      event.set('projectId', currentProjectId)
      event.set('provider', params.attribution?.provider ?? 'unknown')

      try {
        const { mediaLibraryService } = await loadMediaLibraryService()
        const metadata = await mediaLibraryService.importLottieFromUrl(
          params.url,
          currentProjectId,
          { fileName: params.fileName, attribution: params.attribution },
        )

        if (metadata.isDuplicate) {
          showImportNotifications(0, [metadata.fileName], [], 0, get)
          event.success({ imported: 0, duplicates: 1, failed: 0, unsupportedCodecs: 0 })
          return metadata
        }

        prependImportedMedia(set, metadata)
        showImportNotifications(1, [], [], 0, get)
        event.success({ imported: 1, duplicates: 0, failed: 0, unsupportedCodecs: 0 })
        return metadata
      } catch (error) {
        const importError = error instanceof Error ? error : new Error(String(error))
        set({ error: importError.message, errorLink: null })
        event.failure(importError)
        return null
      }
    },

    importFiles: (files: File[]) => importFilesInternal(files),

    importFilesForPlacement: async (files: File[]) =>
      importFilesInternal(files, {
        includeDuplicatesInResults: true,
        waitForPreparation: true,
      }),
  }
}
