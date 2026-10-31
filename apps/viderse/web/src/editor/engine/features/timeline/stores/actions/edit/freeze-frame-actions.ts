import { useItemsStore } from '../../items-store'
import { useTimelineSettingsStore } from '../../timeline-settings-store'
import { useSelectionStore } from '@editor/shared/state/selection'
import { useMediaLibraryStore } from '@editor/features/timeline/deps/media-library-store'
import { importMediaLibraryService } from '@editor/features/timeline/deps/media-library-service'
import { mediaSourceUrl } from '@editor/infrastructure/storage/workspace-fs/media-source'
import { getLogger } from '../shared'
import { timelineToSourceFrames } from '../../../utils/source-calculations'
import { isInTransitionOverlap } from './transition-overlap'
import { runEditCommand } from './command-bridge'
import type { FreezeFrameMutationResult } from './freeze-frame-mutations'

/**
 * Insert a freeze frame at the playhead position.
 *
 * Extracts the video frame at the current playhead, stores it as a media entry,
 * splits the video clip at the playhead, and inserts a still image between the halves.
 *
 * This is async because frame extraction requires mediabunny. The timeline
 * mutations are batched in a single command for undo/redo atomicity.
 */
export async function insertFreezeFrame(itemId: string, playheadFrame: number): Promise<boolean> {
  const items = useItemsStore.getState().items
  const item = items.find((i) => i.id === itemId)
  if (!item || item.type !== 'video') return false

  // Validate playhead is within item bounds (exclusive of edges — need room to split)
  const itemStart = item.from
  const itemEnd = item.from + item.durationInFrames
  if (playheadFrame <= itemStart || playheadFrame >= itemEnd) return false

  // Block freeze frame insertion inside transition overlap zones
  if (isInTransitionOverlap(itemId, playheadFrame - itemStart, item.durationInFrames)) {
    return false
  }

  const fps = useTimelineSettingsStore.getState().fps
  const speed = item.speed ?? 1
  const sourceStart = item.sourceStart ?? 0
  const sourceFps = item.sourceFps ?? fps

  // Calculate source frame at playhead in source-native FPS
  const timelineOffset = playheadFrame - itemStart
  const sourceFrame = sourceStart + timelineToSourceFrames(timelineOffset, speed, fps, sourceFps)

  // Get media metadata for resolution and fps info
  const media = item.mediaId ? useMediaLibraryStore.getState().mediaById[item.mediaId] : undefined
  if (!media) {
    getLogger().error('[insertFreezeFrame] Media not found for item:', item.mediaId)
    return false
  }

  // Calculate timestamp in seconds for frame extraction
  const mediaFps = media.fps || 30
  const timestampSeconds = sourceFrame / mediaFps

  try {
    const { mediaLibraryService } = await importMediaLibraryService()

    // Step 1: Get the media file blob
    const blob = await mediaLibraryService.getMediaFile(media.id)
    if (!blob) {
      getLogger().error('[insertFreezeFrame] Could not access media file')
      return false
    }

    // Step 2: Extract frame using mediabunny at native resolution
    const { Input, BlobSource, CanvasSink, ALL_FORMATS } = await import('mediabunny')
    const input = new Input({
      source: new BlobSource(blob as File),
      formats: ALL_FORMATS,
    })

    const videoTrack = await input.getPrimaryVideoTrack()
    if (!videoTrack) {
      input.dispose()
      getLogger().error('[insertFreezeFrame] No video track found')
      return false
    }

    const frameWidth = videoTrack.displayWidth
    const frameHeight = videoTrack.displayHeight

    const sink = new CanvasSink(videoTrack, {
      width: frameWidth,
      height: frameHeight,
      fit: 'fill',
    })

    const wrapped = await sink.getCanvas(timestampSeconds)
    if (!wrapped) {
      ;(sink as unknown as { dispose?: () => void }).dispose?.()
      input.dispose()
      getLogger().error('[insertFreezeFrame] Failed to extract frame')
      return false
    }

    const canvas = wrapped.canvas as OffscreenCanvas | HTMLCanvasElement
    let frameBlob: Blob
    if ('convertToBlob' in canvas) {
      frameBlob = await canvas.convertToBlob({ type: 'image/png' })
    } else {
      frameBlob = await new Promise<Blob>((resolve, reject) => {
        canvas.toBlob(
          (b) => (b ? resolve(b) : reject(new Error('Failed to create blob'))),
          'image/png',
        )
      })
    }

    // Clean up mediabunny resources
    ;(sink as unknown as { dispose?: () => void }).dispose?.()
    input.dispose()

    // Persist the generated frame before storing its durable media URL in the command.
    const currentProjectId = useMediaLibraryStore.getState().currentProjectId
    if (!currentProjectId) {
      getLogger().error('[insertFreezeFrame] No project context')
      return false
    }

    const fileName = `freeze-frame-${item.label || 'video'}-${Math.round(timestampSeconds * 100) / 100}s.png`
    const frameFile = new File([frameBlob], fileName, {
      type: 'image/png',
      lastModified: Date.now(),
    })

    const mediaMetadata = await mediaLibraryService.importGeneratedImage(
      frameFile,
      currentProjectId,
      {
        width: frameWidth,
        height: frameHeight,
        tags: ['freeze-frame'],
        codec: 'png',
      },
    )
    const frameMediaId = mediaMetadata.id
    const frameSourceUrl = mediaSourceUrl(mediaMetadata)

    // Step 4: Perform timeline mutations atomically (split + insert + shift).
    // Prepend the media item to the store only after the command succeeds so a
    // refused/failed split (e.g. the source clip was removed between validation
    // and commit) doesn't leave an orphaned entry in the media library UI.
    const freezeDurationFrames = Math.round(fps * 2) // 2 seconds

    const outcome = runEditCommand<FreezeFrameMutationResult | null>('edit.insertFreezeFrame', {
      itemId,
      playheadFrame,
      freezeDurationFrames,
      image: {
        label: fileName,
        mediaId: frameMediaId,
        src: frameSourceUrl,
        sourceWidth: frameWidth,
        sourceHeight: frameHeight,
      },
    })
    const committed = outcome.ok ? outcome.result : null
    const success = committed !== null
    if (committed) {
      useSelectionStore.getState().selectItems([committed.freezeFrameItemId])
      useTimelineSettingsStore.getState().markDirty()
    } else {
      getLogger().error('[insertFreezeFrame] Split failed')
    }

    if (!success) {
      // Roll back the newly persisted frame when the atomic timeline edit is refused.
      try {
        await mediaLibraryService.deleteMediaFromProject(currentProjectId, frameMediaId)
      } catch (cleanupError) {
        getLogger().warn(
          '[insertFreezeFrame] Failed to roll back persisted frame after split failure',
          cleanupError,
        )
      }
      return false
    }

    useMediaLibraryStore.getState().prependMediaItem(mediaMetadata)
    return true
  } catch (error) {
    getLogger().error('[insertFreezeFrame] Failed:', error)
    return false
  }
}
