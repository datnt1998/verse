import type { MediaAttribution, MediaMetadata, ThumbnailData } from '@editor/types/storage'
import { createLogger } from '@editor/shared/logging/logger'
import { getEditorStorage } from '@editor/storage/storage-backend'
import { publishMediaSourceChanged } from '@editor/storage/media-source-refresh'

const logger = createLogger('MediaLibraryService')

import {
  getAllMedia as getAllMediaDB,
  getMedia as getMediaDB,
  updateMedia as updateMediaDB,
  deleteMedia as deleteMediaDB,
  getThumbnailByMediaId,
  getThumbnailsByMediaIds,
  deleteThumbnailsByMediaId,
  associateMediaWithProject,
  removeMediaBatchFromProject as removeMediaBatchFromProjectDB,
  removeMediaFromProject as removeMediaFromProjectDB,
  getProjectMediaIds,
  getProjectsUsingMedia,
  getMediaForProject as getMediaForProjectDB,
  deleteTranscript,
  readAiOutput,
  saveCaptions,
  deleteCaptions,
  deleteScenes,
  mediaSourceUrl,
  replaceMediaSource,
} from '@editor/features/media-library/deps/storage'
import {
  importFilmstripCache,
  importGifFrameCache,
  importWaveformCache,
} from '@editor/features/media-library/deps/timeline-services'
import { proxyService } from './proxy-service'
import { enqueueBackgroundMediaWork } from './background-media-work'
import {
  getGeneratedImageDimensions,
  getThumbnailDimensions,
  mediaMetadataToUpdate,
  persistGeneratedMediaAsset,
  saveMediaThumbnail,
} from './media-asset-helpers'
import { validateMediaFileContent, getMimeType, isLottieMime } from '../utils/validation'
import { parseLottieFileBytes } from '@editor/infrastructure/lottie/lottie-metadata'
import { getSharedProxyKey } from '../utils/proxy-key'
import { mediaProcessorService } from './media-processor-service'
import { generateThumbnail } from '../utils/thumbnail-generator'
import {
  needsCustomAudioDecoder,
  startPreviewAudioConform,
  startPreviewAudioStartupWarm,
  deletePreviewAudioConform,
} from '@editor/features/media-library/deps/composition-runtime'

const IMPORT_BACKGROUND_WARM_DELAY_MS = 600
const IMPORT_BACKGROUND_HEAVY_DELAY_MS = 2200
const PAGE_URL_IMPORT_HOSTS = [
  'youtube.com',
  'www.youtube.com',
  'm.youtube.com',
  'youtu.be',
  'vimeo.com',
  'www.vimeo.com',
  'dailymotion.com',
  'www.dailymotion.com',
]

const MIME_TYPE_TO_EXTENSION: Record<string, string> = {
  'video/mp4': '.mp4',
  'video/webm': '.webm',
  'video/quicktime': '.mov',
  'video/x-matroska': '.mkv',
  'video/matroska': '.mkv',
  'video/x-msvideo': '.avi',
  'audio/mp3': '.mp3',
  'audio/mpeg': '.mp3',
  'audio/wav': '.wav',
  'audio/aac': '.aac',
  'audio/x-m4a': '.m4a',
  'audio/mp4': '.m4a',
  'audio/ogg': '.ogg',
  'image/jpeg': '.jpg',
  'image/jpg': '.jpg',
  'image/png': '.png',
  'image/gif': '.gif',
  'image/webp': '.webp',
  'image/svg+xml': '.svg',
}

function normalizeMimeType(value: string | null | undefined): string {
  return value?.split(';')[0]?.trim().toLowerCase() ?? ''
}

function isPageMimeType(mimeType: string): boolean {
  return mimeType.startsWith('text/') || mimeType === 'application/xhtml+xml'
}

function isKnownMediaPageHost(hostname: string): boolean {
  const normalized = hostname.toLowerCase()
  return PAGE_URL_IMPORT_HOSTS.some(
    (host) => normalized === host || normalized.endsWith(`.${host}`),
  )
}

function extractContentDispositionFileName(contentDisposition: string | null): string | null {
  if (!contentDisposition) return null

  const utf8Match = contentDisposition.match(/filename\*\s*=\s*UTF-8''([^;]+)/i)
  const rawUtf8Name = utf8Match?.[1]?.trim()
  if (rawUtf8Name) {
    try {
      return decodeURIComponent(rawUtf8Name.replace(/^"(.*)"$/, '$1'))
    } catch {
      return rawUtf8Name.replace(/^"(.*)"$/, '$1')
    }
  }

  const basicMatch = contentDisposition.match(/filename\s*=\s*("?)([^";]+)\1/i)
  return basicMatch?.[2]?.trim() || null
}

function extractFileNameFromUrl(input: string): string | null {
  try {
    const url = new URL(input)
    const segment = url.pathname.split('/').filter(Boolean).pop()
    if (!segment) return null
    return decodeURIComponent(segment)
  } catch {
    return null
  }
}

function inferExtensionFromMimeType(mimeType: string): string {
  return MIME_TYPE_TO_EXTENSION[mimeType] ?? ''
}

function buildImportedUrlFileName(
  requestedUrl: string,
  responseUrl: string | undefined,
  contentDisposition: string | null,
  mimeType: string,
): string {
  const fileName =
    extractContentDispositionFileName(contentDisposition) ??
    extractFileNameFromUrl(responseUrl ?? requestedUrl) ??
    extractFileNameFromUrl(requestedUrl)

  if (fileName && fileName.length > 0) {
    return fileName
  }

  const extension = inferExtensionFromMimeType(mimeType)
  return `remote-media${extension}`
}

function parseMediaImportUrl(input: string): URL {
  let parsedUrl: URL
  try {
    parsedUrl = new URL(input)
  } catch {
    throw new Error('Enter a valid http:// or https:// media URL.')
  }

  if (parsedUrl.protocol !== 'http:' && parsedUrl.protocol !== 'https:') {
    throw new Error('Only http:// and https:// media URLs are supported.')
  }

  return parsedUrl
}

/**
 * Turn a provider-supplied animation title into a safe base file name.
 * Strips filesystem-hostile characters, collapses whitespace, and caps the
 * length so metadata stays tidy. Falls back to a generic name when empty.
 */
function sanitizeLottieFileName(rawName: string | undefined): string {
  const cleaned = (rawName ?? '')
    .trim()
    .replace(/[/\\?%*:|"<>]/g, '')
    .replace(/\s+/g, ' ')
    .slice(0, 80)
    .trim()
  return cleaned.length > 0 ? cleaned : 'lottie-animation'
}

/**
 * Media Library Service - Coordinates server-owned media (source bytes, probe record, tags,
 * attribution) with browser-derived metadata, thumbnails, and derived caches.
 *
 * Includes in-memory thumbnail URL cache to prevent flicker on re-renders.
 */

/**
 * Decode an audio blob to read its exact duration in seconds. Falls back to the
 * provided value when Web Audio is unavailable or decoding fails.
 */
async function decodeAudioDurationSeconds(blob: Blob, fallbackSeconds: number): Promise<number> {
  const safeFallback = Math.max(0, fallbackSeconds)
  const Ctor =
    typeof window !== 'undefined'
      ? (window.AudioContext ??
        (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext)
      : undefined
  if (!Ctor) return safeFallback

  const ctx = new Ctor()
  try {
    const buffer = await blob.arrayBuffer()
    const audio = await ctx.decodeAudioData(buffer)
    return audio.duration > 0 ? audio.duration : safeFallback
  } catch (error) {
    logger.warn('decodeAudioData failed; using recorder timer duration', error)
    return safeFallback
  } finally {
    void ctx.close().catch(() => {})
  }
}

type ProbedVideoMetadata = Extract<
  Awaited<ReturnType<typeof mediaProcessorService.processMedia>>['metadata'],
  { type: 'video' }
>

function resolveGeneratedThumbnailSize(
  thumbnail: Blob | undefined,
  dimensions: { width: number; height: number },
): { width: number; height: number } | undefined {
  if (!thumbnail) return undefined
  return getThumbnailDimensions(
    Math.max(1, dimensions.width || 1),
    Math.max(1, dimensions.height || 1),
    320,
  )
}

/**
 * `options.fps` wins over the probed rate: the caller (frame interpolation) computes the
 * output rate exactly, which beats estimating it back off the encoded packet timestamps.
 */
function buildGeneratedVideoMetadata(
  file: File,
  mimeType: string,
  metadata: ProbedVideoMetadata,
  options?: { fps?: number; tags?: string[] },
): MediaMetadata {
  const requestedFps = options?.fps
  const fps =
    Number.isFinite(requestedFps) && (requestedFps ?? 0) > 0 ? requestedFps! : metadata.fps
  const createdAt = Date.now()

  return {
    id: crypto.randomUUID(),
    storageType: 'workspace',
    fileName: file.name,
    fileSize: file.size,
    mimeType,
    duration: metadata.duration,
    width: metadata.width,
    height: metadata.height,
    fps,
    codec: metadata.codec,
    bitrate: metadata.bitrate ?? 0,
    audioCodec: metadata.audioCodec,
    audioCodecSupported: metadata.audioCodecSupported,
    videoCodecSupported: metadata.videoCodecSupported,
    keyframeTimestamps: metadata.keyframeTimestamps,
    gopInterval: metadata.gopInterval,
    tags: options?.tags ?? [],
    createdAt,
    updatedAt: createdAt,
  }
}

class MediaLibraryService {
  /**
   * In-memory cache for thumbnail blob URLs to prevent flicker on re-renders.
   * `marker` is the media's `thumbnailId` change-token at the time the URL was
   * created; a changed marker invalidates the entry so regenerated thumbnails
   * are re-read from disk instead of served stale.
   */
  private thumbnailUrlCache = new Map<string, { url: string; marker: string | undefined }>()
  private preparationPromises = new Map<string, Set<Promise<void>>>()

  async waitForMediaPreparation(mediaIds: string[]): Promise<void> {
    const promises = mediaIds.flatMap((mediaId) => [
      ...(this.preparationPromises.get(mediaId) ?? []),
    ])
    if (promises.length === 0) return
    await Promise.allSettled(promises)
  }

  /**
   * Parse a Lottie JSON file into the intrinsic fields needed to populate a
   * `MediaMetadata` record. Lottie can't go through the metadata worker's
   * `createImageBitmap` path (it's JSON, not a rasterizable image), so this
   * runs on the main thread — mirroring the SVG main-thread thumbnail fallback.
   *
   * Uses the WASM-free `lottie-metadata` module (fflate unzip for `.lottie`),
   * so no dotlottie-web WASM is pulled into the import path.
   */
  private async parseLottieFile(
    file: File,
  ): Promise<{ width: number; height: number; fps: number; duration: number }> {
    // Read raw bytes so we can handle both `.json` Lottie and `.lottie`
    // (dotLottie ZIP archive) — `parseLottieFileBytes` auto-detects the format.
    const bytes = new Uint8Array(await file.arrayBuffer())
    const meta = parseLottieFileBytes(bytes)
    if (!meta) {
      throw new Error(`Not a valid Lottie animation: ${file.name}`)
    }

    return {
      width: meta.width,
      height: meta.height,
      fps: meta.frameRate,
      duration: meta.durationSeconds,
    }
  }

  /**
   * Render a representative frame of a Lottie to a thumbnail blob (fit within
   * 320px, preserving aspect). Lazily imports the dotlottie-web renderer so the
   * WASM isn't pulled into the import path unless a Lottie is actually imported.
   */
  private async generateLottieThumbnailBlob(
    file: File,
    width: number,
    height: number,
  ): Promise<{ blob: Blob; width: number; height: number } | undefined> {
    try {
      const dims = getThumbnailDimensions(Math.max(1, width), Math.max(1, height), 320)
      const url = URL.createObjectURL(file)
      try {
        const { renderLottieThumbnail } =
          await import('@editor/infrastructure/lottie/lottie-frame-provider')
        const blob = await renderLottieThumbnail(url, dims.width, dims.height)
        return blob ? { blob, width: dims.width, height: dims.height } : undefined
      } finally {
        URL.revokeObjectURL(url)
      }
    } catch (error) {
      logger.warn('Failed to generate Lottie thumbnail:', error)
      return undefined
    }
  }

  private async deleteTranscriptSafely(mediaId: string): Promise<void> {
    try {
      await deleteTranscript(mediaId)
    } catch (error) {
      logger.warn('Failed to delete transcript:', error)
    }
  }

  private async deleteCaptionsSafely(mediaId: string): Promise<void> {
    try {
      await deleteCaptions(mediaId)
    } catch (error) {
      logger.warn('Failed to delete captions:', error)
    }
  }

  private async deleteScenesSafely(mediaId: string): Promise<void> {
    try {
      await deleteScenes(mediaId)
    } catch (error) {
      logger.warn('Failed to delete scenes:', error)
    }
  }

  private async deleteThumbnailsSafely(mediaId: string): Promise<void> {
    this.clearThumbnailCache(mediaId)
    try {
      await deleteThumbnailsByMediaId(mediaId)
    } catch (error) {
      logger.warn('Failed to delete thumbnails:', error)
    }
  }

  private async clearGifFrameCacheSafely(mediaId: string): Promise<void> {
    try {
      const { gifFrameCache } = await importGifFrameCache()
      await gifFrameCache.clearMedia(mediaId)
    } catch (error) {
      logger.warn('Failed to delete GIF frame cache:', error)
    }
  }

  /**
   * Clear the filmstrip cache for a fully-dereferenced media item. Removes
   * the OPFS cache copy so nothing of the media lingers after the last project using it is gone.
   */
  private async clearFilmstripCacheSafely(mediaId: string): Promise<void> {
    try {
      const { filmstripCache } = await importFilmstripCache()
      await filmstripCache.clearMedia(mediaId)
    } catch (error) {
      logger.warn('Failed to delete filmstrip cache:', error)
    }
  }

  /**
   * Clear waveform caches for a fully-dereferenced media item. Removes
   * the in-memory LRU entry, the persisted binned waveform cache, and the
   * OPFS multi-resolution caches.
   */
  private async clearWaveformCacheSafely(mediaId: string): Promise<void> {
    try {
      const { waveformCache } = await importWaveformCache()
      await waveformCache.clearMedia(mediaId)
    } catch (error) {
      logger.warn('Failed to delete waveform cache:', error)
    }
  }

  private async deleteProxySafely(media: MediaMetadata): Promise<void> {
    try {
      // The proxy key embeds the media id and content hash, so it is never shared between media.
      await proxyService.deleteProxy(media.id, getSharedProxyKey(media))
    } catch (error) {
      logger.warn('Failed to delete proxy:', error)
    } finally {
      proxyService.clearProxyKey(media.id)
    }
  }

  private async cleanupMediaIfUnreferenced(media: MediaMetadata): Promise<void> {
    const remainingProjects = await getProjectsUsingMedia(media.id)

    if (remainingProjects.length > 0) {
      return
    }

    await deleteMediaDB(media.id)

    await this.deleteTranscriptSafely(media.id)
    await this.deleteCaptionsSafely(media.id)
    await this.deleteScenesSafely(media.id)
    await this.deleteThumbnailsSafely(media.id)
    await this.clearGifFrameCacheSafely(media.id)
    await this.clearFilmstripCacheSafely(media.id)
    await this.clearWaveformCacheSafely(media.id)
    await deletePreviewAudioConform(media)
    await this.deleteProxySafely(media)
  }

  private schedulePostImportWork(
    file: File,
    mediaMetadata: MediaMetadata,
    options: {
      isVideo: boolean
      previewAudioCodec?: string
    },
  ): void {
    if (needsCustomAudioDecoder(options.previewAudioCodec)) {
      enqueueBackgroundMediaWork(() => startPreviewAudioStartupWarm(mediaMetadata.id, file), {
        priority: 'warm',
        delayMs: IMPORT_BACKGROUND_WARM_DELAY_MS,
      })
      enqueueBackgroundMediaWork(() => startPreviewAudioConform(mediaMetadata.id, file), {
        priority: 'heavy',
        delayMs: IMPORT_BACKGROUND_HEAVY_DELAY_MS,
      })
    }

    if (mediaMetadata.mimeType === 'image/gif') {
      enqueueBackgroundMediaWork(
        async () => {
          const blobUrl = URL.createObjectURL(file)
          try {
            const { gifFrameCache } = await importGifFrameCache()
            await gifFrameCache.getGifFrames(mediaMetadata.id, blobUrl)
          } finally {
            URL.revokeObjectURL(blobUrl)
          }
        },
        {
          priority: 'warm',
          delayMs: IMPORT_BACKGROUND_WARM_DELAY_MS,
        },
      )
    }
  }

  /**
   * Browser-side analysis of a source file: the metadata the server's probe cannot provide
   * (codec support, keyframes, thumbnail) plus the client view of intrinsic fields. Pure with
   * respect to storage — nothing is written; callers persist the result.
   */
  private async analyzeMediaFile(
    file: File,
    options?: { tags?: string[]; attribution?: MediaAttribution; signal?: AbortSignal },
  ): Promise<{
    mediaMetadata: MediaMetadata
    thumbnailBlob: Blob | undefined
    thumbnailWidth: number | undefined
    thumbnailHeight: number | undefined
    isVideo: boolean
    previewAudioCodec: string | undefined
    hasUnsupportedCodec: boolean
  }> {
    const resolvedMimeType = getMimeType(file)
    const tags = options?.tags ?? []

    // Lottie is JSON/ZIP, not a rasterizable image — parse metadata and render a
    // thumbnail on the main thread (mirrors the SVG fallback), skipping the worker.
    if (isLottieMime(resolvedMimeType)) {
      const lottie = await this.parseLottieFile(file)
      const thumb = await this.generateLottieThumbnailBlob(file, lottie.width, lottie.height)
      const createdAt = Date.now()
      return {
        mediaMetadata: {
          id: crypto.randomUUID(),
          storageType: 'workspace',
          fileName: file.name,
          fileSize: file.size,
          mimeType: resolvedMimeType,
          duration: lottie.duration,
          width: lottie.width,
          height: lottie.height,
          fps: lottie.fps,
          codec: 'lottie',
          bitrate: 0,
          tags,
          attribution: options?.attribution,
          createdAt,
          updatedAt: createdAt,
        },
        thumbnailBlob: thumb?.blob,
        thumbnailWidth: thumb?.width,
        thumbnailHeight: thumb?.height,
        isVideo: false,
        previewAudioCodec: undefined,
        hasUnsupportedCodec: false,
      }
    }

    const { metadata, thumbnail } = await mediaProcessorService.processMedia(
      file,
      resolvedMimeType,
      { thumbnailTimestamp: 1, fastMetadata: true, signal: options?.signal },
    )

    let thumbnailBlob = thumbnail
    if (!thumbnailBlob && resolvedMimeType === 'image/svg+xml') {
      try {
        thumbnailBlob = await generateThumbnail(file, { maxSize: 320, quality: 0.6 })
      } catch (error) {
        logger.warn('Failed to generate SVG thumbnail on main thread:', error)
      }
    }

    const createdAt = Date.now()
    const codecCheck = mediaProcessorService.hasUnsupportedAudioCodec(metadata)
    const previewAudioCodec =
      metadata.type === 'audio'
        ? metadata.codec
        : metadata.type === 'video'
          ? metadata.audioCodec
          : undefined

    const thumbnailDimensions = thumbnailBlob
      ? metadata.type === 'audio'
        ? { width: 320, height: 180 }
        : getThumbnailDimensions(
            Math.max(1, 'width' in metadata ? metadata.width || 1 : 1),
            Math.max(1, 'height' in metadata ? metadata.height || 1 : 1),
            320,
          )
      : undefined

    const mediaMetadata: MediaMetadata = {
      id: crypto.randomUUID(),
      storageType: 'workspace',
      fileName: file.name,
      fileSize: file.size,
      mimeType: resolvedMimeType,
      duration: 'duration' in metadata ? metadata.duration : 0,
      width: 'width' in metadata ? metadata.width : 0,
      height: 'height' in metadata ? metadata.height : 0,
      fps: metadata.type === 'video' ? metadata.fps : 0,
      codec:
        metadata.type === 'video'
          ? metadata.codec
          : metadata.type === 'audio'
            ? metadata.codec || 'unknown'
            : 'unknown',
      bitrate: 'bitrate' in metadata ? (metadata.bitrate ?? 0) : 0,
      audioCodec: metadata.type === 'video' ? metadata.audioCodec : undefined,
      audioCodecSupported: metadata.type === 'video' ? metadata.audioCodecSupported : true,
      videoCodecSupported: metadata.type === 'video' ? metadata.videoCodecSupported : true,
      keyframeTimestamps: metadata.type === 'video' ? metadata.keyframeTimestamps : undefined,
      gopInterval: metadata.type === 'video' ? metadata.gopInterval : undefined,
      tags,
      attribution: options?.attribution,
      createdAt,
      updatedAt: createdAt,
    }

    return {
      mediaMetadata,
      thumbnailBlob,
      thumbnailWidth: thumbnailDimensions?.width,
      thumbnailHeight: thumbnailDimensions?.height,
      isVideo: metadata.type === 'video',
      previewAudioCodec,
      hasUnsupportedCodec: codecCheck.unsupported,
    }
  }

  /**
   * Import an in-memory microphone recording (webm/opus, ogg/opus, or mp4) into
   * server storage for the timeline voiceover feature. Returns metadata
   * whose `duration` is always finite and positive.
   *
   * Mirrors the audio branch of {@link analyzeMediaFile} but is resilient to
   * headerless `MediaRecorder` output, whose container duration frequently probes
   * as `0`/`Infinity` and would otherwise poison downstream waveform/trim math.
   * Duration is resolved once, here, from the best available source:
   * mediabunny probe → sample-accurate `decodeAudioData` → the recorder's wall
   * clock (`fallbackDurationMs`). The caller then reuses the returned `duration`
   * instead of decoding again.
   */
  async importRecordedAudio(
    file: File,
    projectId: string,
    options: { fallbackDurationMs: number },
  ): Promise<MediaMetadata> {
    if (!projectId) {
      throw new Error('No project selected')
    }

    // Strip codec parameters (MediaRecorder yields `audio/webm;codecs=opus`) and
    // fall back to a WebM/Opus container so the media library classifies the
    // take as audio rather than "unknown".
    const rawMimeType = file.type || getMimeType(file)
    const resolvedMimeType = (rawMimeType.split(';')[0]?.trim() || 'audio/webm').replace(
      /^video\/webm$/,
      'audio/webm',
    )

    let probedDuration = 0
    let probedCodec = 'opus'
    let probedBitrate = 0
    let thumbnailBlob: Blob | undefined
    try {
      const { metadata, thumbnail } = await mediaProcessorService.processMedia(
        file,
        resolvedMimeType,
        { fastMetadata: true },
      )
      probedDuration = 'duration' in metadata ? metadata.duration : 0
      if (metadata.type === 'audio' && metadata.codec) {
        probedCodec = metadata.codec
      }
      probedBitrate = 'bitrate' in metadata ? (metadata.bitrate ?? 0) : 0
      thumbnailBlob = thumbnail
    } catch (error) {
      logger.warn('Failed to probe recorded audio; deriving duration by decoding', error)
    }

    let duration = Number.isFinite(probedDuration) && probedDuration > 0 ? probedDuration : 0
    if (duration <= 0) {
      // Probe was unusable (common for headerless WebM) — decode for an exact
      // duration, then fall back to the recorder's wall-clock timer.
      duration = await decodeAudioDurationSeconds(file, options.fallbackDurationMs / 1000)
    }

    const mediaId = crypto.randomUUID()
    const createdAt = Date.now()

    const mediaMetadata: MediaMetadata = {
      id: mediaId,
      storageType: 'workspace',
      fileName: file.name,
      fileSize: file.size,
      mimeType: resolvedMimeType,
      duration,
      width: 0,
      height: 0,
      fps: 0,
      codec: probedCodec,
      bitrate: probedBitrate,
      tags: ['voiceover'],
      createdAt,
      updatedAt: createdAt,
    }

    const persistedMedia = await persistGeneratedMediaAsset({
      file,
      projectId,
      mediaMetadata,
      thumbnailBlob,
      thumbnailWidth: thumbnailBlob ? 320 : undefined,
      thumbnailHeight: thumbnailBlob ? 180 : undefined,
    })

    // Schedules waveform decode so the clip renders its waveform like any audio.
    this.schedulePostImportWork(file, persistedMedia, {
      isVideo: false,
      previewAudioCodec: probedCodec,
    })

    return persistedMedia
  }

  private async fetchMediaFromUrl(url: string): Promise<File> {
    const parsedUrl = parseMediaImportUrl(url.trim())

    let response: Response
    try {
      response = await fetch(parsedUrl.toString())
    } catch (error) {
      logger.warn(`Failed to fetch media URL "${parsedUrl.toString()}":`, error)
      throw new Error(
        'Could not download that URL. The site may block cross-origin downloads, require sign-in, or need a direct file link.',
      )
    }

    if (!response.ok) {
      throw new Error(
        `Failed to download media (${response.status}${response.statusText ? ` ${response.statusText}` : ''}).`,
      )
    }

    const responseMimeType = normalizeMimeType(response.headers.get('content-type'))
    if (isPageMimeType(responseMimeType)) {
      if (isKnownMediaPageHost(parsedUrl.hostname)) {
        throw new Error(
          'YouTube and similar page URLs are not direct media files here yet. Paste a direct MP4/MP3/image URL, or download the media first.',
        )
      }
      throw new Error(
        'That URL points to a web page, not a media file. Paste the direct file URL instead.',
      )
    }

    const blob = await response.blob()
    if (blob.size === 0) {
      throw new Error('The downloaded file was empty.')
    }

    const mimeType = normalizeMimeType(blob.type) || responseMimeType
    const fileName = buildImportedUrlFileName(
      parsedUrl.toString(),
      response.url,
      response.headers.get('content-disposition'),
      mimeType,
    )

    return new File([blob], fileName, {
      type: mimeType || blob.type,
      lastModified: Date.now(),
    })
  }

  /**
   * Get all media items from workspace storage
   */
  async getAllMedia(): Promise<MediaMetadata[]> {
    return getAllMediaDB()
  }

  /**
   * Get a single media item by ID
   */
  async getMedia(id: string): Promise<MediaMetadata | null> {
    const media = await getMediaDB(id)
    return media || null
  }

  /**
   * Import a user-selected file: validate it, analyse it in the browser, upload the bytes to
   * the server (which probes, hashes and stores them), then store the browser-derived extras.
   *
   * The `File` is uploaded as a disk-backed Blob, so large sources stream from disk without
   * being read into JS memory.
   *
   * Duplicate detection: when `projectId` is given and a file with the same name and size is
   * already linked to the project, the existing media is returned with `isDuplicate: true`.
   */
  async importFile(
    file: File,
    options: {
      projectId?: string
      signal?: AbortSignal
      onProgress?: (sent: number, total: number) => void
      tags?: string[]
      attribution?: MediaAttribution
    } = {},
  ): Promise<MediaMetadata & { isDuplicate?: boolean; hasUnsupportedCodec?: boolean }> {
    const { projectId, signal, onProgress, tags, attribution } = options

    const validationResult = await validateMediaFileContent(file)
    if (!validationResult.valid) {
      throw new Error(validationResult.error)
    }

    if (projectId) {
      const projectMedia = await getMediaForProjectDB(projectId)
      const existingMedia = projectMedia.find(
        (media) => media.fileName === file.name && media.fileSize === file.size,
      )
      if (existingMedia) {
        this.schedulePostImportWork(file, existingMedia, {
          isVideo: existingMedia.mimeType.startsWith('video/'),
          previewAudioCodec: existingMedia.mimeType.startsWith('audio/')
            ? existingMedia.codec
            : existingMedia.audioCodec,
        })
        return { ...existingMedia, isDuplicate: true }
      }
    }

    signal?.throwIfAborted()
    const analysis = await this.analyzeMediaFile(file, { tags, attribution, signal })
    signal?.throwIfAborted()

    const persistedMedia = await persistGeneratedMediaAsset({
      file,
      projectId,
      mediaMetadata: analysis.mediaMetadata,
      thumbnailBlob: analysis.thumbnailBlob,
      thumbnailWidth: analysis.thumbnailWidth,
      thumbnailHeight: analysis.thumbnailHeight,
      signal,
      onProgress,
    })

    this.schedulePostImportWork(file, persistedMedia, {
      isVideo: analysis.isVideo,
      previewAudioCodec: analysis.previewAudioCodec,
    })

    return { ...persistedMedia, hasUnsupportedCodec: analysis.hasUnsupportedCodec }
  }

  /**
   * Import several files one after another. A failing file is logged and skipped so one
   * unreadable file does not abort the batch.
   */
  async importFiles(
    files: File[],
    options: {
      projectId?: string
      signal?: AbortSignal
      onFileStart?: (current: number, total: number, fileName: string) => void
      onProgress?: (sent: number, total: number) => void
    } = {},
  ): Promise<MediaMetadata[]> {
    const results: MediaMetadata[] = []
    const errors: { file: string; error: string }[] = []

    for (const [index, file] of files.entries()) {
      options.signal?.throwIfAborted()
      options.onFileStart?.(index + 1, files.length, file.name)

      try {
        results.push(
          await this.importFile(file, {
            projectId: options.projectId,
            signal: options.signal,
            onProgress: options.onProgress,
          }),
        )
      } catch (error) {
        if (options.signal?.aborted) throw error
        errors.push({
          file: file.name,
          error: error instanceof Error ? error.message : String(error),
        })
      }
    }

    if (errors.length > 0) {
      logger.warn('Some files failed to import:', errors)
    }

    return results
  }

  /**
   * Import media from a direct URL.
   *
   * The browser downloads the bytes, then uploads them to the server like any other file.
   *
   * Works with direct media URLs that allow browser-side fetches. Page URLs
   * such as YouTube/Vimeo watch pages typically do not expose a fetchable
   * media file and should be handled upstream.
   */
  async importMediaFromUrl(
    url: string,
    projectId: string,
  ): Promise<MediaMetadata & { isDuplicate?: boolean; hasUnsupportedCodec?: boolean }> {
    if (!projectId) {
      throw new Error('No project selected')
    }

    const file = await this.fetchMediaFromUrl(url)
    return this.importFile(file, { projectId })
  }

  /**
   * Import a Lottie animation from a direct `.lottie`/`.json` URL into
   * server storage. Unlike {@link importMediaFromUrl}, the file is named
   * from `fileName` (so the library shows a human-readable title instead of a
   * CDN hash) and `attribution` is persisted for licensing/credits.
   *
   * The provider CDN must allow cross-origin fetches (LottieFiles serves
   * `Access-Control-Allow-Origin: *`).
   */
  async importLottieFromUrl(
    url: string,
    projectId: string,
    options?: { fileName?: string; attribution?: MediaAttribution },
  ): Promise<MediaMetadata & { isDuplicate?: boolean }> {
    if (!projectId) {
      throw new Error('No project selected')
    }

    let response: Response
    try {
      response = await fetch(url)
    } catch (error) {
      logger.warn(`Failed to fetch Lottie URL "${url}":`, error)
      throw new Error('Could not download that animation. The source may be offline.')
    }

    if (!response.ok) {
      throw new Error(
        `Failed to download animation (${response.status}${response.statusText ? ` ${response.statusText}` : ''}).`,
      )
    }

    const blob = await response.blob()
    if (blob.size === 0) {
      throw new Error('The downloaded animation was empty.')
    }

    const isJson = url.split(/[?#]/)[0]?.toLowerCase().endsWith('.json') ?? false
    const fileName = `${sanitizeLottieFileName(options?.fileName)}.${isJson ? 'json' : 'lottie'}`
    const file = new File([blob], fileName, { type: 'application/lottie+json' })

    return this.importFile(file, { projectId, attribution: options?.attribution })
  }

  /**
   * Save a generated still image into a project-backed media library entry.
   *
   * Used for editor-generated assets such as preview frame captures.
   */
  async importGeneratedImage(
    file: File,
    projectId: string,
    options?: {
      width?: number
      height?: number
      tags?: string[]
      thumbnailMaxSize?: number
      thumbnailQuality?: number
      codec?: string
    },
  ): Promise<MediaMetadata> {
    if (!projectId) {
      throw new Error('No project selected')
    }

    const resolvedMimeType = file.type || getMimeType(file)
    if (!resolvedMimeType.startsWith('image/')) {
      throw new Error(`Generated file must be an image. Received "${resolvedMimeType}".`)
    }

    const safeWidth =
      Number.isFinite(options?.width) && (options?.width ?? 0) > 0
        ? Math.round(options?.width ?? 0)
        : 0
    const safeHeight =
      Number.isFinite(options?.height) && (options?.height ?? 0) > 0
        ? Math.round(options?.height ?? 0)
        : 0
    const dimensions =
      safeWidth > 0 && safeHeight > 0
        ? { width: safeWidth, height: safeHeight }
        : await getGeneratedImageDimensions(file)

    const mediaId = crypto.randomUUID()
    const createdAt = Date.now()
    const codec = options?.codec ?? resolvedMimeType.split('/')[1] ?? 'unknown'
    const thumbnailMaxSize = options?.thumbnailMaxSize ?? 320
    const thumbnailQuality = options?.thumbnailQuality ?? 0.6

    const mediaMetadata: MediaMetadata = {
      id: mediaId,
      storageType: 'workspace',
      fileName: file.name,
      fileSize: file.size,
      mimeType: resolvedMimeType,
      duration: 0,
      width: dimensions.width,
      height: dimensions.height,
      fps: 0,
      codec,
      bitrate: 0,
      tags: options?.tags ?? [],
      createdAt,
      updatedAt: createdAt,
    }

    let thumbnailBlob: Blob | undefined
    let thumbnailDimensions: { width: number; height: number } | undefined
    try {
      thumbnailBlob = await generateThumbnail(file, {
        maxSize: thumbnailMaxSize,
        quality: thumbnailQuality,
      })
      thumbnailDimensions = getThumbnailDimensions(
        dimensions.width,
        dimensions.height,
        thumbnailMaxSize,
      )
    } catch (error) {
      logger.warn(`Failed to save generated image thumbnail for ${file.name}:`, error)
    }

    return persistGeneratedMediaAsset({
      file,
      projectId,
      mediaMetadata,
      thumbnailBlob,
      thumbnailWidth: thumbnailDimensions?.width,
      thumbnailHeight: thumbnailDimensions?.height,
    })
  }

  /**
   * Save a generated video (e.g. a frame-interpolated render) into the project media library
   * as a server-stored asset.
   *
   * Unlike the regular import paths this probes with `fastMetadata: false`. The fast probe
   * hard-codes `fps: 30`, which would silently mislabel a 120fps render as 30fps — the
   * timeline reads `media.fps` as the source frame rate, so every interpolated frame would be
   * skipped. `options.fps` overrides the probe outright when the caller already knows the
   * exact rate, which is more reliable than estimating it back off the encoded packets.
   */
  async importGeneratedVideo(
    file: File,
    projectId: string,
    options?: { fps?: number; tags?: string[] },
  ): Promise<MediaMetadata> {
    if (!projectId) {
      throw new Error('No project selected')
    }

    const resolvedMimeType = file.type || getMimeType(file)
    if (!resolvedMimeType.startsWith('video/')) {
      throw new Error(`Generated file must be a video. Received "${resolvedMimeType}".`)
    }

    const { metadata, thumbnail } = await mediaProcessorService.processMedia(
      file,
      resolvedMimeType,
      { thumbnailTimestamp: 1, fastMetadata: false },
    )
    if (metadata.type !== 'video') {
      throw new Error(`Generated file did not probe as video (got "${metadata.type}").`)
    }

    const thumbnailDimensions = resolveGeneratedThumbnailSize(thumbnail, metadata)
    const mediaMetadata = buildGeneratedVideoMetadata(file, resolvedMimeType, metadata, options)

    return persistGeneratedMediaAsset({
      file,
      projectId,
      mediaMetadata,
      thumbnailBlob: thumbnail,
      thumbnailWidth: thumbnailDimensions?.width,
      thumbnailHeight: thumbnailDimensions?.height,
    })
  }

  /**
   * Save generated audio into the project media library as a server-stored asset.
   */
  async importGeneratedAudio(
    file: File,
    projectId: string,
    options?: {
      tags?: string[]
      thumbnailMaxSize?: number
      thumbnailQuality?: number
      codec?: string
    },
  ): Promise<MediaMetadata> {
    if (!projectId) {
      throw new Error('No project selected')
    }

    const resolvedMimeType = file.type || getMimeType(file)
    if (!resolvedMimeType.startsWith('audio/')) {
      throw new Error(`Generated file must be audio. Received "${resolvedMimeType}".`)
    }

    const thumbnailMaxSize = options?.thumbnailMaxSize ?? 320
    const thumbnailQuality = options?.thumbnailQuality ?? 0.6
    const { metadata, thumbnail } = await mediaProcessorService.processMedia(
      file,
      resolvedMimeType,
      {
        generateThumbnail: true,
        thumbnailMaxSize,
        thumbnailQuality,
      },
    )

    if (metadata.type !== 'audio') {
      throw new Error(`Expected generated audio metadata, received "${metadata.type}".`)
    }

    const mediaId = crypto.randomUUID()
    const createdAt = Date.now()
    const codec = options?.codec ?? metadata.codec ?? resolvedMimeType.split('/')[1] ?? 'unknown'
    // Nominal height — audio waveform thumbnails don't have intrinsic dimensions,
    // so we use a 16:9 placeholder ratio for the DB record.
    const thumbnailHeight = Math.max(1, Math.round(thumbnailMaxSize * (9 / 16)))
    const mediaMetadata: MediaMetadata = {
      id: mediaId,
      storageType: 'workspace',
      fileName: file.name,
      fileSize: file.size,
      mimeType: resolvedMimeType,
      duration: metadata.duration,
      width: 0,
      height: 0,
      fps: 0,
      codec,
      bitrate: metadata.bitrate ?? 0,
      tags: options?.tags ?? [],
      createdAt,
      updatedAt: createdAt,
    }

    return persistGeneratedMediaAsset({
      file,
      projectId,
      mediaMetadata,
      thumbnailBlob: thumbnail,
      thumbnailWidth: thumbnail ? thumbnailMaxSize : undefined,
      thumbnailHeight: thumbnail ? thumbnailHeight : undefined,
    })
  }

  /**
   * Delete media from a project with reference counting
   *
   * Removes the media association from the project. If no other projects use this media,
   * the media (source bytes, metadata, tags) is deleted from the server and its caches cleared.
   *
   * @param projectId - The project to remove media from
   * @param mediaId - The media to remove
   */
  async deleteMediaFromProject(projectId: string, mediaId: string): Promise<void> {
    // Get media metadata
    const media = await getMediaDB(mediaId)
    if (!media) {
      throw new Error(`Media not found: ${mediaId}`)
    }

    // Remove project-media association
    await removeMediaFromProjectDB(projectId, mediaId)

    await this.cleanupMediaIfUnreferenced(media)
  }

  /**
   * Delete multiple media items from a project in batch.
   * Unlinks the whole batch from the project in one write, then cleans up any
   * media no longer referenced by another project.
   */
  async deleteMediaBatchFromProject(projectId: string, mediaIds: string[]): Promise<void> {
    const uniqueMediaIds = Array.from(new Set(mediaIds.filter(Boolean)))
    if (uniqueMediaIds.length === 0) {
      return
    }

    // Snapshot metadata first so missing-media failures preserve old behavior.
    const mediaById = new Map<string, MediaMetadata>()
    const errors: Array<{ id: string; error: unknown }> = []
    for (const mediaId of uniqueMediaIds) {
      try {
        const media = await getMediaDB(mediaId)
        if (!media) {
          throw new Error(`Media not found: ${mediaId}`)
        }
        mediaById.set(mediaId, media)
      } catch (error) {
        logger.error(`Failed to load media ${mediaId} for project delete:`, error)
        errors.push({ id: mediaId, error })
      }
    }

    if (errors.length === uniqueMediaIds.length) {
      throw new Error(
        `Failed to delete all ${uniqueMediaIds.length} items. Check console for details.`,
      )
    }

    const unlinkIds = uniqueMediaIds.filter((id) => mediaById.has(id))
    await removeMediaBatchFromProjectDB(projectId, unlinkIds)

    // After the atomic unlink, slow cleanup can stay serialized to avoid races
    // on reference checks and cache deletes.
    for (const mediaId of unlinkIds) {
      try {
        const media = mediaById.get(mediaId)
        if (!media) continue
        await this.cleanupMediaIfUnreferenced(media)
      } catch (error) {
        logger.error(`Failed to delete media ${mediaId}:`, error)
        errors.push({ id: mediaId, error })
      }
    }

    if (errors.length === uniqueMediaIds.length) {
      throw new Error(
        `Failed to delete all ${uniqueMediaIds.length} items. Check console for details.`,
      )
    }

    if (errors.length > 0) {
      logger.warn(
        `Partially deleted: ${uniqueMediaIds.length - errors.length}/${uniqueMediaIds.length} items deleted successfully.`,
      )
    }
  }

  /**
   * Delete all media associations for a project.
   * Used when deleting a project. Uses parallel deletion for better performance.
   */
  async deleteAllMediaFromProject(projectId: string): Promise<void> {
    const mediaIds = await getProjectMediaIds(projectId)

    for (const mediaId of mediaIds) {
      try {
        await this.deleteMediaFromProject(projectId, mediaId)
      } catch (error) {
        logger.error(`Failed to delete media ${mediaId} from project:`, error)
      }
    }
  }

  /**
   * Delete a media item globally — removes it from every project that uses
   * it, then deletes its server record and source bytes, thumbnails, transcripts and proxies.
   *
   * Prefer `deleteMediaFromProject(projectId, mediaId)` when a project
   * context exists: it preserves the media for other projects.
   * Use this variant only from the no-project view
   * (global media library), or when the user explicitly wants a
   * "delete everywhere" action.
   */
  async deleteMedia(id: string): Promise<void> {
    const media = await getMediaDB(id)
    if (!media) {
      throw new Error(`Media not found: ${id}`)
    }

    // Clean up all project-media associations for this media
    const projectIds = await getProjectsUsingMedia(id)
    for (const projectId of projectIds) {
      try {
        await removeMediaFromProjectDB(projectId, id)
      } catch (error) {
        logger.warn(`Failed to remove project association for ${projectId}:`, error)
      }
    }

    await this.deleteThumbnailsSafely(id)
    await this.clearGifFrameCacheSafely(id)
    await this.clearFilmstripCacheSafely(id)
    await this.clearWaveformCacheSafely(id)
    await deletePreviewAudioConform(media)
    await this.deleteProxySafely(media)

    await deleteMediaDB(id)

    await this.deleteTranscriptSafely(id)
    await this.deleteCaptionsSafely(id)
    await this.deleteScenesSafely(id)
  }

  /**
   * Batch variant of `deleteMedia` — see its docs for when to use this
   * vs. `deleteMediaBatchFromProject`.
   */
  async deleteMediaBatch(ids: string[]): Promise<void> {
    const errors: Array<{ id: string; error: unknown }> = []

    for (const id of ids) {
      try {
        await this.deleteMedia(id)
      } catch (error) {
        logger.error(`Failed to delete media ${id}:`, error)
        errors.push({ id, error })
      }
    }

    if (errors.length === ids.length) {
      throw new Error(`Failed to delete all ${ids.length} items. Check console for details.`)
    }

    if (errors.length > 0) {
      logger.warn(
        `Partially deleted: ${ids.length - errors.length}/${ids.length} items deleted successfully.`,
      )
    }
  }

  /**
   * Get all media for a specific project
   */
  async getMediaForProject(projectId: string): Promise<MediaMetadata[]> {
    return getMediaForProjectDB(projectId)
  }

  /**
   * Link existing media to another project. The server owns one copy of the bytes; linking
   * never duplicates them.
   */
  async copyMediaToProject(mediaId: string, targetProjectId: string): Promise<void> {
    const media = await getMediaDB(mediaId)
    if (!media) {
      throw new Error(`Media not found: ${mediaId}`)
    }

    await associateMediaWithProject(targetProjectId, mediaId)
  }

  /**
   * Fetch a media's source bytes as a Blob. Only for work that needs real bytes (decoding,
   * proxy generation, export); previews should use {@link getMediaUrl}.
   * Returns null when the server no longer has the media.
   */
  async getMediaFile(idOrMedia: string | MediaMetadata): Promise<Blob | null> {
    const media = typeof idOrMedia === 'string' ? await getMediaDB(idOrMedia) : idOrMedia
    if (!media) return null

    const response = await fetch(mediaSourceUrl(media))
    if (response.status === 404) {
      logger.error('Media source is missing on the server:', media.id)
      return null
    }
    if (!response.ok) {
      throw new Error(`Failed to read "${media.fileName}" (${response.status}).`)
    }
    const blob = await response.blob()
    return blob.type === media.mimeType || !media.mimeType
      ? blob
      : new Blob([blob], { type: media.mimeType })
  }

  /**
   * Replace a media's bytes with `file` (relink). The id stays, the content hash changes, and
   * every derivative of the previous bytes (thumbnail, waveform, filmstrip, proxy, conform) is
   * discarded and recomputed from the new file. The file is analysed before the upload so an
   * unreadable file never replaces a working source.
   */
  async relinkMediaFile(
    mediaId: string,
    file: File,
    options?: { signal?: AbortSignal; onProgress?: (sent: number, total: number) => void },
  ): Promise<MediaMetadata> {
    const media = await getMediaDB(mediaId)
    if (!media) {
      throw new Error(`Media not found: ${mediaId}`)
    }

    const validationResult = await validateMediaFileContent(file)
    if (!validationResult.valid) {
      throw new Error(validationResult.error)
    }

    options?.signal?.throwIfAborted()
    const analysis = await this.analyzeMediaFile(file, { signal: options?.signal })
    await replaceMediaSource(mediaId, file, {
      mimeType: analysis.mediaMetadata.mimeType,
      ...options,
    })

    await this.deleteThumbnailsSafely(mediaId)
    await this.clearGifFrameCacheSafely(mediaId)
    await this.clearFilmstripCacheSafely(mediaId)
    await this.clearWaveformCacheSafely(mediaId)
    await deletePreviewAudioConform(media)
    await this.deleteProxySafely(media)

    const updates = mediaMetadataToUpdate(analysis.mediaMetadata)
    if (analysis.thumbnailBlob) {
      const thumbnailId = await saveMediaThumbnail(
        mediaId,
        analysis.thumbnailBlob,
        analysis.thumbnailWidth,
        analysis.thumbnailHeight,
      )
      if (thumbnailId) updates.thumbnailId = thumbnailId
    }
    const relinked = await updateMediaDB(mediaId, updates)

    this.schedulePostImportWork(file, relinked, {
      isVideo: analysis.isVideo,
      previewAudioCodec: analysis.previewAudioCodec,
    })
    if (typeof window !== 'undefined' && relinked.contentHash) {
      publishMediaSourceChanged({ workspaceId: await getEditorStorage().workspaceId(), mediaId, contentHash: relinked.contentHash })
    }
    return relinked
  }

  /**
   * Update AI-generated captions for a media item.
   *
   * Captions live in `cache/ai/captions.json` as the authoritative source.
   * We also mirror them onto `MediaMetadata.aiCaptions` so in-memory zustand
   * consumers and search (`media-library-store.ts`) don't need a separate
   * hydration pass — the mirror stays consistent because this is the only
   * writer.
   */
  async updateMediaCaptions(
    mediaId: string,
    captions: NonNullable<MediaMetadata['aiCaptions']>,
    options?: {
      service?: string
      model?: string
      sampleIntervalSec?: number
      embeddingModel?: string
      embeddingDim?: number
      imageEmbeddingModel?: string
      imageEmbeddingDim?: number
      /**
       * When provided, the captions envelope and heavy assets are mirrored
       * into the shared content-addressable cache so other mediaIds with the
       * same source bytes skip re-analysis.
       */
      contentHash?: string
    },
  ): Promise<MediaMetadata> {
    const existingEnvelope = await readAiOutput(mediaId, 'captions').catch(() => undefined)
    const existingSampleInterval = (() => {
      const fromData = existingEnvelope?.data.sampleIntervalSec
      if (typeof fromData === 'number') return fromData
      const fromParams = (existingEnvelope?.params as { sampleIntervalSec?: unknown } | undefined)
        ?.sampleIntervalSec
      return typeof fromParams === 'number' ? fromParams : undefined
    })()

    const resolvedOptions = {
      service: options?.service ?? existingEnvelope?.service ?? 'lfm-captioning',
      model: options?.model ?? existingEnvelope?.model ?? 'lfm-2.5-vl',
      sampleIntervalSec: options?.sampleIntervalSec ?? existingSampleInterval,
      embeddingModel: options?.embeddingModel ?? existingEnvelope?.data.embeddingModel,
      embeddingDim: options?.embeddingDim ?? existingEnvelope?.data.embeddingDim,
      imageEmbeddingModel:
        options?.imageEmbeddingModel ?? existingEnvelope?.data.imageEmbeddingModel,
      imageEmbeddingDim: options?.imageEmbeddingDim ?? existingEnvelope?.data.imageEmbeddingDim,
      contentHash: options?.contentHash ?? existingEnvelope?.data.contentHash,
    }

    try {
      await saveCaptions({
        mediaId,
        captions,
        service: resolvedOptions.service,
        model: resolvedOptions.model,
        sampleIntervalSec: resolvedOptions.sampleIntervalSec,
        embeddingModel: resolvedOptions.embeddingModel,
        embeddingDim: resolvedOptions.embeddingDim,
        imageEmbeddingModel: resolvedOptions.imageEmbeddingModel,
        imageEmbeddingDim: resolvedOptions.imageEmbeddingDim,
        contentHash: resolvedOptions.contentHash,
      })
    } catch (error) {
      logger.warn(
        `Failed to persist captions for ${mediaId}; metadata mirror will still update`,
        error,
      )
    }
    return updateMediaDB(mediaId, { aiCaptions: captions })
  }

  /**
   * Direct URL of a media's current bytes for preview/playback. The URL embeds the content
   * hash, so replacing the bytes yields a new URL, and the browser range-requests it instead
   * of holding the file in memory.
   */
  async getMediaUrl(idOrMedia: string | MediaMetadata): Promise<string | null> {
    const media = typeof idOrMedia === 'string' ? await getMediaDB(idOrMedia) : idOrMedia
    if (!media) return null
    return mediaSourceUrl(media)
  }

  /**
   * Get thumbnail for a media item
   */
  async getThumbnail(mediaId: string): Promise<ThumbnailData | null> {
    const thumbnail = await getThumbnailByMediaId(mediaId)
    return thumbnail || null
  }

  /**
   * Get thumbnail as blob URL (cached in memory to prevent flicker)
   */
  async getThumbnailBlobUrl(mediaId: string, thumbnailId?: string): Promise<string | null> {
    // Serve from cache only when the change-marker still matches. A caller
    // without a marker (undefined) accepts whatever is cached; a caller with a
    // marker that differs falls through to a fresh read.
    const cached = this.thumbnailUrlCache.get(mediaId)
    if (cached && (thumbnailId === undefined || cached.marker === thumbnailId)) {
      return cached.url
    }

    const thumbnail = await this.getThumbnail(mediaId)

    if (!thumbnail) {
      return null
    }

    // The prefetch batch (or another caller) may have populated the cache
    // during the await above — reuse it when its marker matches rather than
    // leaking a second URL.
    const raced = this.thumbnailUrlCache.get(mediaId)
    if (raced && (thumbnailId === undefined || raced.marker === thumbnailId)) {
      return raced.url
    }

    return this.cacheThumbnailUrl(mediaId, thumbnail.blob, thumbnailId)
  }

  /**
   * Store a thumbnail blob URL for `mediaId`, revoking any prior URL (e.g. a
   * stale entry from a superseded `thumbnailId`) so it can't leak.
   */
  private cacheThumbnailUrl(mediaId: string, blob: Blob, marker: string | undefined): string {
    const existing = this.thumbnailUrlCache.get(mediaId)
    if (existing) {
      URL.revokeObjectURL(existing.url)
    }
    const url = URL.createObjectURL(blob)
    this.thumbnailUrlCache.set(mediaId, { url, marker })
    return url
  }

  /**
   * Warm the in-memory thumbnail-URL cache for many media in one batched pass.
   *
   * Called on project load so each card's mount is a synchronous cache hit
   * instead of an independent async cache read. Only fetches ids not already
   * cached with a matching change-marker.
   */
  async prefetchThumbnails(items: Array<{ id: string; thumbnailId?: string }>): Promise<void> {
    const markerById = new Map(items.map((item) => [item.id, item.thumbnailId]))
    const uncached = items
      .filter((item) => {
        const cached = this.thumbnailUrlCache.get(item.id)
        return !cached || cached.marker !== item.thumbnailId
      })
      .map((item) => item.id)
    if (uncached.length === 0) return

    const blobs = await getThumbnailsByMediaIds(uncached)
    for (const [mediaId, blob] of blobs) {
      const marker = markerById.get(mediaId)
      // A concurrent getThumbnailBlobUrl() may have already cached this id with
      // the same marker while the batch was in flight — don't re-create it.
      const cached = this.thumbnailUrlCache.get(mediaId)
      if (cached && cached.marker === marker) continue
      this.cacheThumbnailUrl(mediaId, blob, marker)
    }
  }

  /**
   * Clear thumbnail URL from cache (call when media is deleted)
   */
  clearThumbnailCache(mediaId: string): void {
    const cached = this.thumbnailUrlCache.get(mediaId)
    if (cached) {
      URL.revokeObjectURL(cached.url)
      this.thumbnailUrlCache.delete(mediaId)
    }
  }
}

// Singleton instance
export const mediaLibraryService = new MediaLibraryService()
