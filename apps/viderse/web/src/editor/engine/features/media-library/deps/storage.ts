// Media-library-local adapter for server-backed storage APIs.
//
// Keep direct storage imports out of media-library services; this facade is
// intentionally thin and behavior-preserving. Source bytes, probe records, tags and
// attribution live on the server; thumbnails, waveforms, proxies and other derivatives
// are disposable OPFS caches.
export {
  associateMediaWithProject,
  deleteCaptions,
  deleteMedia,
  deleteScenes,
  deleteThumbnailsByMediaId,
  deleteTranscript,
  getAllMedia,
  getMedia,
  getMediaForProject,
  getProjectMediaIds,
  getProjectsUsingMedia,
  getThumbnailByMediaId,
  getThumbnailsByMediaIds,
  hasMediaSource,
  importMediaSource,
  mediaSourceUrl,
  readAiOutput,
  readMediaSource,
  removeMediaBatchFromProject,
  removeMediaFromProject,
  replaceMediaSource,
  saveCaptions,
  saveThumbnail,
  updateMedia,
  type MediaMetadataUpdate,
} from '@editor/infrastructure/storage'
