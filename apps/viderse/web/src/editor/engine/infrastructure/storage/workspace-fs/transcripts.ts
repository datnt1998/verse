/**
 * Per-media transcripts in the disposable OPFS cache.
 *
 * Persisted at `media/{mediaId}/cache/ai/transcript.json` as an
 * {@link AiOutput} envelope (versioned by the server content hash like every
 * `media/{mediaId}/...` cache path).
 *
 * The public API still exposes {@link MediaTranscript} — the flat record
 * shape predates the envelope and is what the UI and indexers consume.
 */

import type { MediaTranscript } from '@editor/types/storage'
import { createLogger } from '@editor/shared/logging/logger'

import {
  readAiOutput,
  writeAiOutput,
  deleteAiOutput,
  getMediaIdsWithAiOutput,
  transcriptFromLegacy,
  transcriptToLegacy,
} from './ai-outputs'

const logger = createLogger('WorkspaceFS:Transcripts')

export async function getTranscript(mediaId: string): Promise<MediaTranscript | undefined> {
  try {
    const envelope = await readAiOutput(mediaId, 'transcript')
    return envelope ? transcriptToLegacy(envelope) : undefined
  } catch (error) {
    logger.error(`getTranscript(${mediaId}) failed`, error)
    throw new Error(`Failed to load transcript: ${mediaId}`)
  }
}

export async function getTranscriptMediaIds(mediaIds: string[]): Promise<Set<string>> {
  if (mediaIds.length === 0) return new Set()
  try {
    return await getMediaIdsWithAiOutput(mediaIds, 'transcript')
  } catch (error) {
    logger.error('getTranscriptMediaIds failed', error)
    throw new Error('Failed to enumerate transcripts')
  }
}

export async function saveTranscript(transcript: MediaTranscript): Promise<MediaTranscript> {
  try {
    const envelope = transcriptFromLegacy(transcript)
    const written = await writeAiOutput({
      mediaId: envelope.mediaId,
      kind: 'transcript',
      service: envelope.service,
      model: envelope.model,
      params: envelope.params,
      data: envelope.data,
    })
    return transcriptToLegacy(written)
  } catch (error) {
    logger.error(`saveTranscript(${transcript.mediaId}) failed`, error)
    throw new Error(`Failed to save transcript: ${transcript.mediaId}`)
  }
}

export async function deleteTranscript(mediaId: string): Promise<void> {
  try {
    await deleteAiOutput(mediaId, 'transcript')
  } catch (error) {
    logger.error(`deleteTranscript(${mediaId}) failed`, error)
    throw new Error(`Failed to delete transcript: ${mediaId}`)
  }
}
