import { getEditorStorage, setEditorStorage } from '@editor/storage/storage-backend'
import { serverMediaToMetadata } from '@editor/storage/media-adapter'
import { useCompositionsStore } from '@editor/features/timeline/stores/compositions-store'
import { useMediaLibraryStore } from '@editor/features/media-library/stores/media-library-store'
import { resolveMediaUrls } from '@editor/features/timeline/deps/media-library-resolver'
import { processAudioWindows, clearAudioDecodeCache } from '@editor/features/export/utils/canvas-audio'
import { interleavePlanarToF32le } from '@editor/features/export/utils/audio-measurement'
import { compositionFromProject } from '../ui/clone/audio-measurement'
import { cloneExportSettings } from '../ui/export/export-settings'
import { runServerExport } from '../ui/export/export-runner'
import { HeadlessClient, runtimeSchema, withHeadlessLease, type HeadlessProgress } from './headless-client'
import { HeadlessEditorStorage } from './headless-storage'

export type HeadlessOutcome = { exportId: string } | { lufs: number | null; durationSec: number }

/** Both launchers use the saved, revision-bound engine composition, never editor UI state. */
export async function runHeadlessRequest(client: HeadlessClient, controller: AbortController, onProgress: (progress: HeadlessProgress) => void): Promise<HeadlessOutcome> {
  let lastProgress: HeadlessProgress | undefined
  const report = (progress: HeadlessProgress) => { lastProgress = progress; onProgress(progress) }
  const signal = controller.signal
  report({ phase: 'load' })
  const snapshot = await client.snapshot(signal)
  signal.throwIfAborted()
  report({ phase: 'claim' })
  await client.claim(snapshot.request.nonce, signal)
  let outcome: HeadlessOutcome | undefined
  await withHeadlessLease(client, controller, () => lastProgress, async () => {
    if (!snapshot.runtime.ready) throw new Error('Bộ xử lý chưa sẵn sàng')
    if (!snapshot.runtime.capabilities.audioDecode) throw new Error('Bộ xử lý chưa giải mã được âm thanh')
    const storage = new HeadlessEditorStorage(client, snapshot)
    const previousStorage = getEditorStorage()
    signal.throwIfAborted()
    setEditorStorage(storage)
    const restoreStorage = () => { if (getEditorStorage() === storage) setEditorStorage(previousStorage) }
    signal.addEventListener('abort', restoreStorage, { once: true })
    try {
      const { project, revision } = snapshot
      const { composition, compositions } = compositionFromProject(project)
      useCompositionsStore.getState().setCompositions(compositions)
      const mediaItems = snapshot.media.map(serverMediaToMetadata)
      useMediaLibraryStore.setState({ currentProjectId: project.id, mediaItems, mediaById: Object.fromEntries(mediaItems.map(media => [media.id, media])) })
      // Do not retain arbitrary saved URLs: every source is re-resolved inside the scoped snapshot.
      const scopedMedia = new Set(snapshot.media.map(media => media.id))
      for (const items of [project.timeline?.items ?? [], ...compositions.map(nested => nested.items)]) {
        for (const item of items) {
          if (['video', 'audio', 'image', 'lottie'].includes(item.type) && (!item.mediaId || !scopedMedia.has(item.mediaId))) throw new Error('Thiếu tệp nguồn trong phạm vi yêu cầu')
        }
      }
      composition.tracks = await resolveMediaUrls(composition.tracks, { useProxy: false, signal })
      if (client.kind === 'render') {
        report({ phase: 'caps' })
        const adapter = await navigator.gpu?.requestAdapter()
        if (!adapter) throw new Error('Trình duyệt xuất ngầm không có WebGPU')
        if (typeof VideoEncoder === 'undefined') throw new Error('Trình duyệt xuất ngầm thiếu bộ mã hoá video')
        const encoder = await VideoEncoder.isConfigSupported({ codec: 'avc1.640028', width: 1920, height: 1080, framerate: 30 })
        if (!encoder.supported) throw new Error('Trình duyệt xuất ngầm thiếu H.264 1080p30')
        const result = await runServerExport({
          project, revision, composition, settings: cloneExportSettings(), fps: project.metadata.fps,
          durationFrames: composition.durationInFrames ?? Math.max(1, Math.round(project.duration * project.metadata.fps)),
          clone: snapshot.clone ? { jobId: snapshot.clone.jobId } : undefined,
          fetchCloneGate: async () => (await client.json('/runtime', runtimeSchema, { signal })).clone?.gate ?? { allowed: false },
          requestId: client.requestId, signal,
          onExportId: exportId => client.bind(exportId, signal),
          onProgress: progress => {
            if (progress.kind === 'render') report({ phase: 'rendering', framesDone: progress.progress.currentFrame ?? null, framesTotal: progress.progress.totalFrames ?? null })
            else report({ phase: 'uploading', bytesSent: progress.sent, bytesTotal: progress.total })
          },
        })
        return { exportId: result.record.id }
      }

      report({ phase: 'decode', bytesSent: 0, chunksSent: 0 })
      let bytesSent = 0
      let chunksSent = 0
      let samples = 0
      // The same windowed mixer as audio-measurement.worker, executed in this isolated page so
      // nested composition resolution uses the scoped storage too. Uploading each window before
      // decoding the next bounds memory and gives the server contiguous durable chunk sequence.
      for await (const window of processAudioWindows(composition, signal)) {
        if (window.sampleRate !== 48000 || window.channels !== 2) throw new Error('Bộ trộn âm thanh không trả stereo 48 kHz')
        const bytes = interleavePlanarToF32le(window.samples)
        for (let offset = 0; offset < bytes.byteLength; offset += 4 * 1024 ** 2) {
          signal.throwIfAborted()
          const chunk = bytes.subarray(offset, Math.min(offset + 4 * 1024 ** 2, bytes.byteLength))
          await client.uploadPcm(chunksSent, chunk, signal)
          bytesSent += chunk.byteLength
          chunksSent += 1
          report({ phase: 'upload', bytesSent, chunksSent })
        }
        samples += window.samples[0]?.length ?? 0
      }
      report({ phase: 'finish', bytesSent, chunksSent })
      return { lufs: null, durationSec: samples / 48000 }
    } finally {
      clearAudioDecodeCache()
      signal.removeEventListener('abort', restoreStorage)
      restoreStorage()
    }
  }, async result => {
    const completed = await client.complete('exportId' in result ? result.exportId : undefined, signal)
    if ('exportId' in result) outcome = result
    else {
      if (!completed.result) throw new Error('Máy chủ không trả kết quả đo')
      outcome = { lufs: completed.result.lufs, durationSec: completed.result.durationSec }
    }
  })
  if (!outcome) throw new Error('Yêu cầu kết thúc mà không có kết quả')
  return outcome
}
