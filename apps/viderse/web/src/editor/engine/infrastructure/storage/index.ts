/** Server-managed durable storage and disposable, content-versioned OPFS caches. */

// Projects
export {
  getAllProjects,
  getProject,
  createProject,
  updateProject,
  deleteProject,
  duplicateProject,
  ensureProject,
  getProjectSnapshot,
  getProjectRevision,
  acceptServerRevision,
  type NewProjectInput,
} from '@editor/infrastructure/storage/workspace-fs/projects'

// Media
export {
  getAllMedia,
  getMedia,
  updateMedia,
  deleteMedia,
  type MediaMetadataUpdate,
} from '@editor/infrastructure/storage/workspace-fs/media'

// Thumbnails
export {
  saveThumbnail,
  getThumbnailByMediaId,
  getThumbnailsByMediaIds,
  deleteThumbnailsByMediaId,
  saveProjectThumbnail,
  loadProjectThumbnail,
} from '@editor/infrastructure/storage/workspace-fs/thumbnails'


// Project-media associations
export {
  associateMediaWithProject,
  removeMediaFromProject,
  removeMediaBatchFromProject,
  getProjectMediaIds,
  getProjectsUsingMedia,
  getMediaForProject,
} from '@editor/infrastructure/storage/workspace-fs/project-media'

// Waveforms
export {
  getWaveform,
  getWaveformRecord,
  getWaveformMeta,
  getWaveformBins,
  saveWaveformMeta,
  saveWaveformBin,
  deleteWaveform,
} from '@editor/infrastructure/storage/workspace-fs/waveforms'

// GIF frames
export {
  saveGifFrames,
  getGifFrames,
  deleteGifFrames,
  clearAllGifFrames,
} from '@editor/infrastructure/storage/workspace-fs/gif-frames'

// Decoded preview audio
export {
  getDecodedPreviewAudio,
  saveDecodedPreviewAudio,
  deleteDecodedPreviewAudio,
} from '@editor/infrastructure/storage/workspace-fs/decoded-preview-audio'

// Transcripts
export {
  getTranscript,
  getTranscriptMediaIds,
  saveTranscript,
  deleteTranscript,
} from '@editor/infrastructure/storage/workspace-fs/transcripts'

// AI captions (vision-language-model frame descriptions)
export {
  getCaptionsByContentHash,
  saveCaptions,
  adoptCaptionsFromCache,
  deleteCaptions,
  saveCaptionThumbnail,
  getCaptionThumbnailBlob,
  probeCaptionThumbnail,
  deleteCaptionThumbnails,
  saveCaptionEmbeddings,
  getCaptionEmbeddings,
  getCaptionsEmbeddingsMeta,
  deleteCaptionEmbeddings,
  saveCaptionImageEmbeddings,
  getCaptionImageEmbeddings,
} from '@editor/infrastructure/storage/workspace-fs/captions'

// Media source files
export {
  hasMediaSource,
  readMediaSource,
  importMediaSource,
  replaceMediaSource,
  mediaSourceUrl,
} from '@editor/infrastructure/storage/workspace-fs/media-source'

// Workspace cache mirror helpers
export {
  mirrorBlobToWorkspace,
  mirrorJsonToWorkspace,
  readWorkspaceBlob,
  removeWorkspaceCacheEntry,
} from '@editor/infrastructure/storage/workspace-fs/cache-mirror'

// Workspace cache path helpers
export { proxyDir, proxyFilePath, proxyMetaPath } from '@editor/infrastructure/storage/workspace-fs/paths'

// Embedded text-subtitle track cache (parsed once per source fingerprint)
export {
  getEmbeddedSubtitleSidecar,
  saveEmbeddedSubtitleSidecar,
} from '@editor/infrastructure/storage/workspace-fs/embedded-subtitles'

// Scene-detection results
export { deleteScenes } from '@editor/infrastructure/storage/workspace-fs/scenes'

// Generic AI-output envelope (use these directly for new AI services)
export { readAiOutput } from '@editor/infrastructure/storage/workspace-fs/ai-outputs'

// Orphan cache sweep
export {
  sweepWorkspaceOrphans,
  type OrphanSweepReport,
  type OrphanSweepOptions,
} from '@editor/infrastructure/storage/workspace-fs/orphan-sweep'

// Final render outputs (export queue)
export {
  saveExportFile,
  saveRenderedExport,
  listExportFiles,
  readExportFile,
  deleteExportFile,
  workspaceFolderName,
  type ExportFileEntry,
} from '@editor/infrastructure/storage/workspace-fs/exports'

// Per-project render-queue persistence
export {
  loadRenderQueue,
  saveRenderQueue,
} from '@editor/infrastructure/storage/workspace-fs/render-queue'

// User-saved effect presets (grades)
export {
  readUserEffectPresets,
  saveUserEffectPresets,
  type UserEffectPreset,
} from '@editor/infrastructure/storage/workspace-fs/effect-presets'

// Per-project animation presets
export {
  readAnimationPresets,
  saveAnimationPresets,
  sanitizeAnimationPresets,
  type AnimationPreset,
  type AnimationPresetProperty,
  type AnimationPresetVectorProperty,
} from '@editor/infrastructure/storage/workspace-fs/animation-presets'
