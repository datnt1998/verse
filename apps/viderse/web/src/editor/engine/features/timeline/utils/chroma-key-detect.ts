/**
 * Chroma-key auto colour: calls the server-side background detection for an
 * item's media so the effect's `keyColor` can be set to the detected green.
 * Used when a chroma key is added and by the "Tự dò màu nền" button.
 */
import { getEditorStorage } from '@editor/storage/storage-backend'

export const CHROMA_KEY_EFFECT = 'gpu-chroma-key'

/** Run detection for an item's media; resolves to the detected key colour or null. */
export async function detectChromaKeyColorForItem(
  item: { mediaId?: string },
  options: { refresh?: boolean } = {},
): Promise<string | null> {
  if (!item.mediaId) return null
  const media = await getEditorStorage().detectMediaBackground(item.mediaId, { refresh: options.refresh === true })
  return media.background?.kind === 'green' ? (media.background.keyColor ?? null) : null
}
