import type { TimelineItem } from '@editor/types/timeline'
import { importMediaLibraryService } from '@editor/features/timeline/deps/media-library-service'
import { useMediaLibraryStore } from '@editor/features/timeline/deps/media-library-store'
import { useItemsStore } from '../items-store'
import { useTimelineSettingsStore } from '../timeline-settings-store'
import { runEditCommand } from './edit/command-bridge'

async function buildVideoHasAudioMap(
  items: TimelineItem[],
): Promise<Record<string, boolean>> {
  const mediaById = useMediaLibraryStore.getState().mediaById
  const mediaIds = Array.from(
    new Set(
      items
        .filter(
          (item): item is Extract<TimelineItem, { type: 'video' }> =>
            item.type === 'video' && !!item.mediaId,
        )
        .map((item) => item.mediaId!),
    ),
  )

  const entries = await Promise.all(
    mediaIds.map(async (mediaId) => {
      const cachedMedia = mediaById[mediaId]
      if (cachedMedia) {
        return [mediaId, !!cachedMedia.audioCodec] as const
      }

      const { mediaLibraryService } = await importMediaLibraryService()
      const media = await mediaLibraryService.getMedia(mediaId)
      return [mediaId, !!media?.audioCodec] as const
    }),
  )

  return Object.fromEntries(entries)
}

export async function repairLegacyAvTracks(): Promise<boolean> {
  const videoHasAudioByMediaId = await buildVideoHasAudioMap(useItemsStore.getState().items)
  const outcome = runEditCommand<boolean>('edit.repairLegacyAvTracks', {
    videoHasAudioByMediaId,
  })
  if (!outcome.ok || !outcome.result) return false

  useTimelineSettingsStore.getState().markDirty()
  return true
}
