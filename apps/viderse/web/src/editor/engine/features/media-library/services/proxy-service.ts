/**
 * Proxy Video Service
 *
 * Manages 960x540-bounded proxy video generation lifecycle:
 * - Start/cancel proxy generation for a media item
 * - Track generation status per mediaId
 * - Load existing proxies from OPFS on startup
 * - Provide proxy blob URLs for preview playback
 * - Clean up proxies when media is deleted
 *
 * Storage structure (OPFS cache, disposable, versioned by workspace + media + content hash):
 *   <cache>/<workspaceId>/media/<mediaId>/<contentHash>/proxy/
 *     proxy.mp4
 *     meta.json - { width, height, status, createdAt, version, sourceWidth, sourceHeight }
 */

import { createLogger } from '@editor/shared/logging/logger'
import { createManagedWorker } from '@editor/shared/utils/managed-worker'
import {
  registerObjectUrl,
  unregisterObjectUrl,
} from '@editor/infrastructure/browser/object-url-registry'
import { readCacheBlob, removeCacheEntry } from '@editor/storage/opfs-cache'
import {
  PROXY_FILE_NAME,
  PROXY_META_FILE_NAME,
  PROXY_SCHEMA_VERSION,
  proxyOpfsFilePath,
} from '../proxy-constants'
import { parseProxyKey, proxyCacheDirSegments, proxyCacheIdentity } from '../utils/proxy-key'
import type { ProxyWorkerRequest, ProxyWorkerResponse } from '../workers/proxy-generation-worker'
import { enqueueBackgroundMediaWork } from './background-media-work'

const logger = createLogger('ProxyService')

function revokeRegisteredObjectUrl(url: string): void {
  unregisterObjectUrl(url)
  URL.revokeObjectURL(url)
}

interface ProxyMetadata {
  version?: number
  width: number
  height: number
  sourceWidth?: number
  sourceHeight?: number
  status: string
  createdAt: number
  /** Video codec the proxy was encoded with ('avc' | 'hevc'); absent on legacy H.264 proxies. */
  codec?: string
}

// A proxy's codec is recorded in its metadata. A proxy generated on one machine (e.g. an
// HEVC proxy) may not be playable on another (HEVC support is platform-dependent); using it
// would break preview. Gate on `<video>` playback support so an unplayable proxy is skipped
// and the source is used instead. H.264 — and legacy proxies with no recorded codec — are
// universally playable.
let hevcVideoPlayable: boolean | null = null
function canPlayProxyCodec(codec: string | undefined): boolean {
  if (!codec || codec === 'avc') return true
  if (codec === 'hevc') {
    if (hevcVideoPlayable === null) {
      try {
        hevcVideoPlayable =
          typeof document !== 'undefined' &&
          document.createElement('video').canPlayType('video/mp4; codecs="hvc1.1.6.L93.B0"') ===
            'probably'
      } catch {
        hevcVideoPlayable = false
      }
    }
    return hevcVideoPlayable
  }
  return true // Unknown codec — don't block; best effort.
}

type ProxyStatusListener = (
  mediaId: string,
  status: 'generating' | 'ready' | 'error' | 'idle',
  progress?: number,
) => void

interface ProxyPrewarmMedia {
  mimeType: string
  duration: number
}

type ProxyMediaResolver = (mediaId: string) => ProxyPrewarmMedia | undefined
type ProxyFilmstripPrewarm = (
  mediaId: string,
  proxyFile: Blob,
  duration: number,
  window: { startTime: number; endTime: number },
) => void

type ProxySourceLoader = () => Promise<Blob | null>
type ProxySourceInput = Blob | ProxySourceLoader
type ProxyJobPriority = 'user' | 'background'

interface ProxyGenerationOptions {
  priority?: ProxyJobPriority
}

interface QueuedProxySource {
  load: ProxySourceLoader
}

interface QueuedProxyJob {
  mediaId: string
  proxyKey: string
  sourceWidth: number
  sourceHeight: number
  source: QueuedProxySource
  priority: ProxyJobPriority
}

interface ProgressEmissionState {
  lastEmitAt: number
  lastProgress: number
  pendingProgress: number | null
  timeoutId: ReturnType<typeof setTimeout> | null
}

const PROXY_PROGRESS_EMIT_INTERVAL_MS = 150
const PROXY_PROGRESS_EMIT_MIN_DELTA = 0.01
const PROXY_FILMSTRIP_COVER_PREWARM_SECONDS = 1
const PROXY_FILMSTRIP_PREWARM_SECONDS = 12
const PROXY_FILMSTRIP_COVER_PREWARM_DELAY_MS = 0
const PROXY_FILMSTRIP_PREWARM_DELAY_MS = 900
class ProxyService {
  private proxyBlobUrlByKey = new Map<string, string>()
  private proxyKeyByMediaId = new Map<string, string>()
  private mediaIdsByProxyKey = new Map<string, Set<string>>()
  private progressByProxyKey = new Map<string, number>()
  private pendingJobsByKey = new Map<string, QueuedProxyJob>()
  private pendingJobOrder: string[] = []
  private activeJobPhaseByKey = new Map<string, 'loading' | 'processing'>()
  private activeJobPriorityByKey = new Map<string, ProxyJobPriority>()
  private progressEmissionByProxyKey = new Map<string, ProgressEmissionState>()
  private statusListener: ProxyStatusListener | null = null
  private mediaResolver: ProxyMediaResolver | null = null
  private filmstripPrewarm: ProxyFilmstripPrewarm | null = null
  private generatingProxyKeys = new Set<string>()
  private isRefreshing = false
  private readonly maxConcurrentJobs = 1
  private readonly workerManager = createManagedWorker({
    createWorker: () =>
      new Worker(new URL('../workers/proxy-generation-worker.ts', import.meta.url), {
        type: 'module',
      }),
    setupWorker: (worker) => {
      worker.onmessage = (event: MessageEvent<ProxyWorkerResponse>) => {
        this.handleWorkerMessage(event.data)
      }

      worker.onerror = (error) => {
        logger.error('Proxy worker error:', error)
      }

      return () => {
        worker.onmessage = null
        worker.onerror = null
      }
    },
  })

  /**
   * Register a listener for proxy status changes (used by the store)
   */
  onStatusChange(listener: ProxyStatusListener): void {
    this.statusListener = listener
  }

  /**
   * Register a read-only media lookup used for proxy-related cache prewarming.
   */
  setMediaResolver(resolver: ProxyMediaResolver): void {
    this.mediaResolver = resolver
  }

  /**
   * Register optional follow-up work for a ready proxy. Kept outside this
   * service so proxy lifecycle does not statically depend on timeline caches.
   */
  setFilmstripPrewarm(prewarm: ProxyFilmstripPrewarm): void {
    this.filmstripPrewarm = prewarm
  }

  /**
   * Register media -> proxy identity mapping so multiple media items can
   * share one physical proxy file.
   */
  setProxyKey(mediaId: string, proxyKey: string): void {
    const existingKey = this.proxyKeyByMediaId.get(mediaId)
    if (existingKey === proxyKey) {
      return
    }

    if (existingKey) {
      const existingSet = this.mediaIdsByProxyKey.get(existingKey)
      existingSet?.delete(mediaId)
      if (existingSet && existingSet.size === 0) {
        this.mediaIdsByProxyKey.delete(existingKey)
      }
    }

    this.proxyKeyByMediaId.set(mediaId, proxyKey)

    let mediaIds = this.mediaIdsByProxyKey.get(proxyKey)
    if (!mediaIds) {
      mediaIds = new Set<string>()
      this.mediaIdsByProxyKey.set(proxyKey, mediaIds)
    }
    mediaIds.add(mediaId)

    // Immediately synchronize status for newly mapped aliases.
    if (this.proxyBlobUrlByKey.has(proxyKey)) {
      this.statusListener?.(mediaId, 'ready')
    } else if (this.generatingProxyKeys.has(proxyKey)) {
      this.statusListener?.(mediaId, 'generating', this.progressByProxyKey.get(proxyKey) ?? 0)
    }
  }

  /**
   * Remove media -> proxy identity mapping.
   */
  clearProxyKey(mediaId: string): void {
    const proxyKey = this.proxyKeyByMediaId.get(mediaId)
    if (!proxyKey) {
      return
    }

    this.proxyKeyByMediaId.delete(mediaId)
    const mediaIds = this.mediaIdsByProxyKey.get(proxyKey)
    mediaIds?.delete(mediaId)
    if (mediaIds && mediaIds.size === 0) {
      this.mediaIdsByProxyKey.delete(proxyKey)
    }
  }

  getProxyKey(mediaId: string): string | undefined {
    return this.proxyKeyByMediaId.get(mediaId)
  }

  /**
   * Check if media supports manual proxy generation.
   */
  canGenerateProxy(mimeType: string): boolean {
    return mimeType.startsWith('video/')
  }

  /**
   * Start proxy generation for a media item
   */
  generateProxy(
    mediaId: string,
    source: ProxySourceInput,
    sourceWidth: number,
    sourceHeight: number,
    proxyKey?: string,
    options?: ProxyGenerationOptions,
  ): void {
    if (proxyKey) {
      this.setProxyKey(mediaId, proxyKey)
    }
    const resolvedProxyKey = this.resolveProxyKey(mediaId, proxyKey)

    if (this.proxyBlobUrlByKey.has(resolvedProxyKey)) {
      this.emitStatusForProxyKey(resolvedProxyKey, 'ready')
      return
    }

    if (this.generatingProxyKeys.has(resolvedProxyKey)) {
      const existingJob = this.pendingJobsByKey.get(resolvedProxyKey)
      if (
        existingJob &&
        (options?.priority ?? 'user') === 'user' &&
        existingJob.priority !== 'user'
      ) {
        this.removePendingJob(resolvedProxyKey)
        existingJob.priority = 'user'
        this.insertPendingJob(existingJob)
      }
      if ((options?.priority ?? 'user') === 'user') {
        this.activeJobPriorityByKey.set(resolvedProxyKey, 'user')
      }
      this.emitStatusForProxyKey(
        resolvedProxyKey,
        'generating',
        this.progressByProxyKey.get(resolvedProxyKey) ?? 0,
      )
      return
    }

    this.generatingProxyKeys.add(resolvedProxyKey)
    this.progressByProxyKey.set(resolvedProxyKey, 0)
    this.emitStatusForProxyKey(resolvedProxyKey, 'generating', 0)

    this.insertPendingJob({
      mediaId,
      proxyKey: resolvedProxyKey,
      sourceWidth,
      sourceHeight,
      source: this.normalizeSource(source),
      priority: options?.priority ?? 'user',
    })
    this.drainQueue()
  }

  /**
   * Cancel proxy generation for a media item
   */
  cancelProxy(mediaId: string, proxyKey?: string): void {
    const resolvedProxyKey = this.resolveProxyKey(mediaId, proxyKey)
    if (!this.generatingProxyKeys.has(resolvedProxyKey)) return

    this.generatingProxyKeys.delete(resolvedProxyKey)
    this.progressByProxyKey.delete(resolvedProxyKey)
    this.clearProgressEmissionState(resolvedProxyKey)
    this.emitStatusForProxyKey(resolvedProxyKey, 'idle')

    if (this.pendingJobsByKey.has(resolvedProxyKey)) {
      this.removePendingJob(resolvedProxyKey)
      this.activeJobPriorityByKey.delete(resolvedProxyKey)
      return
    }

    const activePhase = this.activeJobPhaseByKey.get(resolvedProxyKey)
    if (!activePhase) {
      return
    }

    if (activePhase === 'loading') {
      this.activeJobPhaseByKey.delete(resolvedProxyKey)
      this.activeJobPriorityByKey.delete(resolvedProxyKey)
      this.drainQueue()
      return
    }

    const worker = this.getWorker()
    worker.postMessage({
      type: 'cancel',
      mediaId: resolvedProxyKey,
    } as ProxyWorkerRequest)
  }

  cancelBackgroundProxy(mediaId: string, proxyKey?: string): void {
    const resolvedProxyKey = this.resolveProxyKey(mediaId, proxyKey)
    const pendingPriority = this.pendingJobsByKey.get(resolvedProxyKey)?.priority
    const activePriority = this.activeJobPriorityByKey.get(resolvedProxyKey)
    if (pendingPriority !== 'background' && activePriority !== 'background') return
    this.cancelProxy(mediaId, resolvedProxyKey)
  }

  /**
   * Get proxy blob URL if available
   */
  getProxyBlobUrl(mediaId: string, proxyKey?: string): string | null {
    const resolvedProxyKey = this.resolveProxyKey(mediaId, proxyKey)
    return this.proxyBlobUrlByKey.get(resolvedProxyKey) ?? null
  }

  getMediaIdByProxyUrl(url: string): string | null {
    for (const [proxyKey, proxyUrl] of this.proxyBlobUrlByKey) {
      if (proxyUrl !== url) continue
      return this.mediaIdsByProxyKey.get(proxyKey)?.values().next().value ?? null
    }
    return null
  }

  /**
   * Check if proxy exists and is ready
   */
  hasProxy(mediaId: string, proxyKey?: string): boolean {
    const resolvedProxyKey = this.resolveProxyKey(mediaId, proxyKey)
    return this.proxyBlobUrlByKey.has(resolvedProxyKey)
  }

  /**
   * Delete the proxy of a media item (OPFS cache + in-memory URL)
   */
  async deleteProxy(mediaId: string, proxyKey?: string): Promise<void> {
    const resolvedProxyKey = this.resolveProxyKey(mediaId, proxyKey)

    // Cancel if generating
    this.cancelProxy(mediaId, resolvedProxyKey)

    this.releaseProxyUrl(resolvedProxyKey)

    if (!parseProxyKey(resolvedProxyKey)) return
    const identity = await proxyCacheIdentity(resolvedProxyKey)
    await removeCacheEntry(['media', identity.mediaId, 'proxy'], true, identity)
    logger.debug(`Deleted proxy for ${resolvedProxyKey}`)
  }

  clearMemory(mediaId: string): void {
    const key = this.resolveProxyKey(mediaId)
    this.cancelProxy(mediaId, key)
    this.releaseProxyUrl(key)
    this.clearProxyKey(mediaId)
  }

  /**
   * Load existing proxies from the OPFS cache at startup.
   * Only loads proxies for the given mediaIds (project-scoped). Returns the media ids whose
   * cached proxy was discarded (interrupted, stale schema, empty or unreadable) and needs
   * regenerating.
   */
  async loadExistingProxies(mediaIds: string[]): Promise<string[]> {
    const staleProxyIds: string[] = []

    const requestedProxyKeys = new Set<string>()
    for (const mediaId of mediaIds) {
      requestedProxyKeys.add(this.resolveProxyKey(mediaId))
    }

    const collectMappedMediaIds = (proxyKey: string): string[] => {
      const mappedMediaIds = this.mediaIdsByProxyKey.get(proxyKey)
      if (!mappedMediaIds || mappedMediaIds.size === 0) {
        return []
      }

      return [...mappedMediaIds]
    }

    const removeProxyEntry = async (proxyKey: string, reason: string): Promise<void> => {
      try {
        const identity = await proxyCacheIdentity(proxyKey)
        await removeCacheEntry(['media', identity.mediaId, 'proxy'], true, identity)
        logger.debug(`Removed ${reason} proxy for ${proxyKey}`)
      } catch (error) {
        // `removeCacheEntry` already treats an absent or read-only cache as done; only the
        // identity lookup can fail here.
        logger.warn(`Failed to remove ${reason} proxy for ${proxyKey}:`, error)
      }
    }

    for (const proxyKey of requestedProxyKeys) {
      // A media without a content hash (never mapped) has no cache identity, hence no proxy.
      if (!parseProxyKey(proxyKey)) continue

      try {
        const metadata = await this.readProxyMetadata(proxyKey)
        if (!metadata) continue

        if (metadata.status === 'generating') {
          staleProxyIds.push(...collectMappedMediaIds(proxyKey))
          await removeProxyEntry(proxyKey, 'interrupted')
          continue
        }

        if (metadata.status === 'error') {
          // Failed proxies are removed, but we do not automatically requeue
          // them on startup. Repeated deterministic failures should not keep
          // restarting in the background every session.
          await removeProxyEntry(proxyKey, 'failed')
          continue
        }

        if (metadata.status !== 'ready') {
          staleProxyIds.push(...collectMappedMediaIds(proxyKey))
          await removeProxyEntry(proxyKey, 'invalid-status')
          continue
        }

        // Invalidate stale proxy formats to avoid carrying forward old proxy
        // dimensions after proxy sizing changes.
        if (metadata.version !== PROXY_SCHEMA_VERSION) {
          staleProxyIds.push(...collectMappedMediaIds(proxyKey))
          await removeProxyEntry(proxyKey, 'stale')
          continue
        }

        // Skip (but keep) a proxy this machine can't play — e.g. an HEVC proxy generated
        // on another machine. Preview falls back to the source; the file stays for
        // machines that can use it.
        if (!canPlayProxyCodec(metadata.codec)) {
          logger.debug(
            `Skipping proxy ${proxyKey}: codec '${metadata.codec}' not playable here; using source`,
          )
          continue
        }

        const proxyFile = await this.readProxyFile(proxyKey)
        if (!proxyFile || proxyFile.size === 0) {
          staleProxyIds.push(...collectMappedMediaIds(proxyKey))
          await removeProxyEntry(proxyKey, 'empty')
          continue
        }

        await this.setProxyUrl(proxyKey, proxyFile)
        this.emitStatusForProxyKey(proxyKey, 'ready')

        logger.debug(`Loaded existing proxy for ${proxyKey}`)
      } catch (error) {
        logger.warn(`Failed to load existing proxy for ${proxyKey}:`, error)
        staleProxyIds.push(...collectMappedMediaIds(proxyKey))
        await removeProxyEntry(proxyKey, 'invalid')
      }
    }

    return staleProxyIds
  }

  /**
   * Get or create the shared worker instance
   */
  private getWorker(): Worker {
    return this.workerManager.getWorker()
  }

  /**
   * Handle messages from the worker
   */
  private handleWorkerMessage(message: ProxyWorkerResponse): void {
    // Worker uses `mediaId` field as proxyKey for storage identity.
    const proxyKey = message.mediaId

    switch (message.type) {
      case 'progress': {
        if (!this.generatingProxyKeys.has(proxyKey)) {
          break
        }
        this.progressByProxyKey.set(proxyKey, message.progress)
        this.emitProgressThrottled(proxyKey, message.progress)
        break
      }

      case 'complete': {
        const wasCancelled = !this.generatingProxyKeys.has(proxyKey)
        this.activeJobPhaseByKey.delete(proxyKey)
        this.activeJobPriorityByKey.delete(proxyKey)
        this.generatingProxyKeys.delete(proxyKey)
        this.progressByProxyKey.delete(proxyKey)
        this.clearProgressEmissionState(proxyKey)
        this.drainQueue()
        if (wasCancelled) {
          break
        }
        // Load the completed proxy from OPFS
        void this.loadCompletedProxy(proxyKey)
        break
      }

      case 'cancelled': {
        this.activeJobPhaseByKey.delete(proxyKey)
        this.activeJobPriorityByKey.delete(proxyKey)
        this.generatingProxyKeys.delete(proxyKey)
        this.progressByProxyKey.delete(proxyKey)
        this.clearProgressEmissionState(proxyKey)
        this.drainQueue()
        break
      }

      case 'error': {
        const wasCancelled = !this.generatingProxyKeys.has(proxyKey)
        this.activeJobPhaseByKey.delete(proxyKey)
        this.activeJobPriorityByKey.delete(proxyKey)
        this.generatingProxyKeys.delete(proxyKey)
        this.progressByProxyKey.delete(proxyKey)
        this.clearProgressEmissionState(proxyKey)
        this.drainQueue()
        if (wasCancelled) {
          break
        }
        logger.error(`Proxy generation failed for ${proxyKey}:`, message.error)
        this.emitStatusForProxyKey(proxyKey, 'error')
        break
      }
    }
  }

  /**
   * Load a freshly completed proxy from OPFS and cache its blob URL
   */
  private async loadCompletedProxy(proxyKey: string): Promise<void> {
    try {
      const proxyFile = await this.readProxyFile(proxyKey)

      if (!proxyFile || proxyFile.size === 0) {
        this.emitStatusForProxyKey(proxyKey, 'error')
        return
      }

      await this.setProxyUrl(proxyKey, proxyFile)
      this.emitStatusForProxyKey(proxyKey, 'ready')
      this.prewarmFilmstripFromProxy(proxyKey, proxyFile)

      logger.debug(`Proxy ready for ${proxyKey}`)
    } catch (error) {
      logger.error(`Failed to load completed proxy for ${proxyKey}:`, error)
      this.emitStatusForProxyKey(proxyKey, 'error')
    }
  }

  private prewarmFilmstripFromProxy(proxyKey: string, proxyFile: Blob): void {
    const mediaIds = [...(this.mediaIdsByProxyKey.get(proxyKey) ?? [])]
    if (mediaIds.length === 0) {
      return
    }

    for (const mediaId of mediaIds) {
      const media = this.mediaResolver?.(mediaId)
      if (!media || !media.mimeType.startsWith('video/') || media.duration <= 0) {
        continue
      }

      const coverWarmEndTime = Math.min(media.duration, PROXY_FILMSTRIP_COVER_PREWARM_SECONDS)
      enqueueBackgroundMediaWork(
        () =>
          this.filmstripPrewarm?.(mediaId, proxyFile, media.duration, {
            startTime: 0,
            endTime: coverWarmEndTime,
          }),
        {
          priority: 'warm',
          delayMs: PROXY_FILMSTRIP_COVER_PREWARM_DELAY_MS,
        },
      )

      const warmEndTime = Math.min(media.duration, PROXY_FILMSTRIP_PREWARM_SECONDS)
      enqueueBackgroundMediaWork(
        () =>
          this.filmstripPrewarm?.(mediaId, proxyFile, media.duration, {
            startTime: 0,
            endTime: warmEndTime,
          }),
        {
          priority: 'warm',
          delayMs: PROXY_FILMSTRIP_PREWARM_DELAY_MS,
        },
      )
    }
  }

  /**
   * Re-read all cached proxy files from OPFS and create fresh blob URLs.
   * Call this after tab wake-up to recover from stale blob URLs caused by
   * browser memory pressure or tab throttling during inactivity.
   *
   * @returns Number of proxy blob URLs that were refreshed
   */
  async refreshAllBlobUrls(): Promise<number> {
    if (this.isRefreshing) return 0

    const proxyKeys = [...this.proxyBlobUrlByKey.keys()]
    if (proxyKeys.length === 0) return 0

    this.isRefreshing = true
    let refreshed = 0

    try {
      for (const proxyKey of proxyKeys) {
        try {
          const proxyFile = await this.readProxyFile(proxyKey)

          if (!proxyFile || proxyFile.size === 0) {
            // The cache entry was evicted or emptied; drop the stale URL.
            this.releaseProxyUrl(proxyKey)
            continue
          }

          await this.setProxyUrl(proxyKey, proxyFile)
          refreshed++
        } catch {
          this.releaseProxyUrl(proxyKey)
        }
      }
    } catch (error) {
      logger.warn('Failed to refresh proxy blob URLs:', error)
    } finally {
      this.isRefreshing = false
    }

    if (refreshed > 0) {
      logger.debug(`Refreshed ${refreshed} proxy blob URLs from OPFS`)
    }

    return refreshed
  }

  private async readProxyFile(proxyKey: string): Promise<Blob | null> {
    const identity = await proxyCacheIdentity(proxyKey)
    return readCacheBlob(['media', identity.mediaId, 'proxy', PROXY_FILE_NAME], identity)
  }

  private async readProxyMetadata(proxyKey: string): Promise<ProxyMetadata | null> {
    const identity = await proxyCacheIdentity(proxyKey)
    const metaFile = await readCacheBlob(
      ['media', identity.mediaId, 'proxy', PROXY_META_FILE_NAME],
      identity,
    )
    if (!metaFile) return null
    return JSON.parse(await metaFile.text()) as ProxyMetadata
  }

  /** Publish `proxyFile` as the proxy's blob URL, revoking the one it replaces. */
  private async setProxyUrl(proxyKey: string, proxyFile: Blob): Promise<void> {
    const opfsPath = proxyOpfsFilePath(await proxyCacheDirSegments(proxyKey))
    const previousUrl = this.proxyBlobUrlByKey.get(proxyKey)
    if (previousUrl) revokeRegisteredObjectUrl(previousUrl)

    const blobUrl = URL.createObjectURL(proxyFile)
    registerObjectUrl(blobUrl, proxyFile, { opfsPath, fileSize: proxyFile.size })
    this.proxyBlobUrlByKey.set(proxyKey, blobUrl)
  }

  private releaseProxyUrl(proxyKey: string): void {
    const url = this.proxyBlobUrlByKey.get(proxyKey)
    if (url) revokeRegisteredObjectUrl(url)
    this.proxyBlobUrlByKey.delete(proxyKey)
  }

  private resolveProxyKey(mediaId: string, explicitProxyKey?: string): string {
    if (explicitProxyKey) {
      return explicitProxyKey
    }
    return this.proxyKeyByMediaId.get(mediaId) ?? mediaId
  }

  private normalizeSource(source: ProxySourceInput): QueuedProxySource {
    if (typeof source === 'function') {
      return { load: source }
    }
    return { load: async () => source }
  }

  private insertPendingJob(job: QueuedProxyJob): void {
    this.pendingJobsByKey.set(job.proxyKey, job)

    const existingIndex = this.pendingJobOrder.indexOf(job.proxyKey)
    if (existingIndex >= 0) {
      this.pendingJobOrder.splice(existingIndex, 1)
    }

    if (job.priority === 'background') {
      this.pendingJobOrder.push(job.proxyKey)
      return
    }

    const firstBackgroundIndex = this.pendingJobOrder.findIndex(
      (proxyKey) => this.pendingJobsByKey.get(proxyKey)?.priority === 'background',
    )

    if (firstBackgroundIndex === -1) {
      this.pendingJobOrder.push(job.proxyKey)
      return
    }

    this.pendingJobOrder.splice(firstBackgroundIndex, 0, job.proxyKey)
  }

  private removePendingJob(proxyKey: string): void {
    this.pendingJobsByKey.delete(proxyKey)
    const pendingIndex = this.pendingJobOrder.indexOf(proxyKey)
    if (pendingIndex >= 0) {
      this.pendingJobOrder.splice(pendingIndex, 1)
    }
  }

  private drainQueue(): void {
    while (
      this.activeJobPhaseByKey.size < this.maxConcurrentJobs &&
      this.pendingJobOrder.length > 0
    ) {
      const nextProxyKey = this.pendingJobOrder.shift()
      if (!nextProxyKey) {
        break
      }

      const job = this.pendingJobsByKey.get(nextProxyKey)
      if (!job) {
        continue
      }

      this.pendingJobsByKey.delete(nextProxyKey)
      void this.runQueuedJob(job)
    }
  }

  private async runQueuedJob(job: QueuedProxyJob): Promise<void> {
    const { proxyKey } = job
    this.activeJobPriorityByKey.set(proxyKey, job.priority)
    try {
      const worker = this.getWorker()

      this.activeJobPhaseByKey.set(proxyKey, 'loading')

      let source: Blob | null = null
      try {
        source = await job.source.load()
      } catch (error) {
        if (!this.generatingProxyKeys.has(proxyKey)) {
          this.activeJobPhaseByKey.delete(proxyKey)
          this.activeJobPriorityByKey.delete(proxyKey)
          this.drainQueue()
          return
        }

        this.failQueuedJob(proxyKey, `Failed to load source for ${proxyKey}`, error)
        return
      }

      if (!this.generatingProxyKeys.has(proxyKey)) {
        this.activeJobPhaseByKey.delete(proxyKey)
        this.activeJobPriorityByKey.delete(proxyKey)
        this.drainQueue()
        return
      }

      if (!source) {
        this.failQueuedJob(proxyKey, `Source media unavailable for ${proxyKey}`)
        return
      }

      const dirSegments = await proxyCacheDirSegments(proxyKey)
      if (!this.generatingProxyKeys.has(proxyKey)) {
        this.activeJobPhaseByKey.delete(proxyKey)
        this.activeJobPriorityByKey.delete(proxyKey)
        this.drainQueue()
        return
      }

      this.activeJobPhaseByKey.set(proxyKey, 'processing')
      worker.postMessage({
        type: 'generate',
        mediaId: proxyKey,
        dirSegments,
        source,
        sourceWidth: job.sourceWidth,
        sourceHeight: job.sourceHeight,
      } satisfies ProxyWorkerRequest)
    } catch (error) {
      this.failQueuedJob(proxyKey, `Failed to start proxy generation for ${proxyKey}`, error)
    }
  }

  private failQueuedJob(proxyKey: string, message: string, error?: unknown): void {
    this.activeJobPhaseByKey.delete(proxyKey)
    this.activeJobPriorityByKey.delete(proxyKey)
    this.generatingProxyKeys.delete(proxyKey)
    this.progressByProxyKey.delete(proxyKey)
    this.clearProgressEmissionState(proxyKey)
    if (error) {
      logger.error(message, error)
    } else {
      logger.error(message)
    }
    this.emitStatusForProxyKey(proxyKey, 'error')
    this.drainQueue()
  }

  private emitProgressThrottled(proxyKey: string, progress: number): void {
    const now = Date.now()
    const state = this.progressEmissionByProxyKey.get(proxyKey) ?? {
      lastEmitAt: 0,
      lastProgress: 0,
      pendingProgress: null,
      timeoutId: null,
    }

    const shouldEmitImmediately =
      progress >= 1 ||
      progress <= 0 ||
      progress - state.lastProgress >= PROXY_PROGRESS_EMIT_MIN_DELTA ||
      now - state.lastEmitAt >= PROXY_PROGRESS_EMIT_INTERVAL_MS

    if (shouldEmitImmediately) {
      if (state.timeoutId !== null) {
        clearTimeout(state.timeoutId)
        state.timeoutId = null
      }
      state.pendingProgress = null
      state.lastEmitAt = now
      state.lastProgress = progress
      this.progressEmissionByProxyKey.set(proxyKey, state)
      this.emitStatusForProxyKey(proxyKey, 'generating', progress)
      return
    }

    state.pendingProgress = progress
    if (state.timeoutId === null) {
      state.timeoutId = setTimeout(() => {
        const pendingState = this.progressEmissionByProxyKey.get(proxyKey)
        if (!pendingState) {
          return
        }

        pendingState.timeoutId = null
        if (pendingState.pendingProgress == null) {
          return
        }

        const nextProgress = pendingState.pendingProgress
        pendingState.pendingProgress = null
        pendingState.lastEmitAt = Date.now()
        pendingState.lastProgress = nextProgress
        this.progressEmissionByProxyKey.set(proxyKey, pendingState)
        this.emitStatusForProxyKey(proxyKey, 'generating', nextProgress)
      }, PROXY_PROGRESS_EMIT_INTERVAL_MS)
    }

    this.progressEmissionByProxyKey.set(proxyKey, state)
  }

  private clearProgressEmissionState(proxyKey: string): void {
    const state = this.progressEmissionByProxyKey.get(proxyKey)
    if (state?.timeoutId != null) {
      clearTimeout(state.timeoutId)
    }
    this.progressEmissionByProxyKey.delete(proxyKey)
  }

  private emitStatusForProxyKey(
    proxyKey: string,
    status: 'generating' | 'ready' | 'error' | 'idle',
    progress?: number,
  ): void {
    const mediaIds = this.mediaIdsByProxyKey.get(proxyKey)
    if (mediaIds && mediaIds.size > 0) {
      for (const mediaId of mediaIds) {
        this.statusListener?.(mediaId, status, progress)
      }
      return
    }

    // Fallback for legacy/unmapped calls where proxyKey == mediaId.
    this.statusListener?.(proxyKey, status, progress)
  }
}

// Singleton
export const proxyService = new ProxyService()
