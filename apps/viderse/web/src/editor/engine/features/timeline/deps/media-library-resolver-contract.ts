export { mediaProcessorService } from '@editor/features/media-library/services/media-processor-service'
export {
  resolveMediaUrl,
  resolveProxyUrl,
  resolveMediaUrls,
} from '@editor/features/media-library/utils/media-resolver'
export {
  getMediaDragData,
  clearMediaDragData,
  type CompositionDragData,
} from '@editor/features/media-library/utils/drag-data-cache'
export {
  extractValidMediaFileEntriesFromDataTransfer,
  formatMediaDropRejectionMessage,
} from '@editor/features/media-library/utils/file-drop'
export type { OrphanedClipInfo } from '@editor/features/media-library/types'
export type { ExtractedMediaFileEntry } from '@editor/features/media-library/utils/file-drop'
export { getMediaType, getMimeType } from '@editor/features/media-library/utils/validation'
