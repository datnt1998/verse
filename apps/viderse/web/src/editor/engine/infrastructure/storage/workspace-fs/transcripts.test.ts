// @vitest-environment node

import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { MediaTranscript } from '@editor/types/storage'
import '../test-utils/logger-test-mocks'

// vi.mock is hoisted above static imports, so the shared factory can only be
// loaded lazily from inside it.
vi.mock('@editor/storage/storage-backend', async () =>
  (await import('./__tests__/in-memory-opfs')).storageBackendMock,
)

import { memoryOpfs } from './__tests__/in-memory-opfs'
import { resetMediaIdentityTracking } from './cache-identity'
import {
  deleteTranscript,
  getTranscript,
  getTranscriptMediaIds,
  saveTranscript,
} from './transcripts'

function makeTranscript(mediaId: string): MediaTranscript {
  return {
    mediaId,
    model: 'whisper-tiny',
    quantization: 'q4',
    language: 'en',
    segments: [{ id: 0, start: 0, end: 1, text: 'hello' }],
    createdAt: 0,
  } as unknown as MediaTranscript
}

beforeEach(() => {
  memoryOpfs.reset()
  resetMediaIdentityTracking()
  for (const id of ['m1', 'm2', 'm3']) memoryOpfs.setMedia(id, `hash-${id}`)
})

describe('workspace-fs transcripts', () => {
  it('saveTranscript then getTranscript round-trips', async () => {
    await saveTranscript(makeTranscript('m1'))
    const t = await getTranscript('m1')
    expect(t!.mediaId).toBe('m1')
    expect(t!.segments[0]!.text).toBe('hello')
  })

  it('files the transcript under the media content hash', async () => {
    await saveTranscript(makeTranscript('m1'))
    const stored = await memoryOpfs.readText(
      memoryOpfs.mediaPathFor('m1', 'hash-m1', 'cache', 'ai', 'transcript.json'),
    )
    expect(stored).not.toBeNull()
  })

  it('getTranscript returns undefined when missing', async () => {
    expect(await getTranscript('m2')).toBeUndefined()
  })

  it('does not serve a transcript of the previous bytes after the source is replaced', async () => {
    await saveTranscript(makeTranscript('m1'))
    memoryOpfs.setMedia('m1', 'hash-m1-replaced')
    expect(await getTranscript('m1')).toBeUndefined()
  })

  it('getTranscript treats a corrupt cache file as a miss', async () => {
    await memoryOpfs.plantText(
      memoryOpfs.mediaPathFor('m1', 'hash-m1', 'cache', 'ai', 'transcript.json'),
      '{not json',
    )
    expect(await getTranscript('m1')).toBeUndefined()
  })

  it('getTranscriptMediaIds returns only ids that have a transcript', async () => {
    await saveTranscript(makeTranscript('m1'))
    await saveTranscript(makeTranscript('m3'))
    const ids = await getTranscriptMediaIds(['m1', 'm2', 'm3'])
    expect(ids).toEqual(new Set(['m1', 'm3']))
  })

  it('getTranscriptMediaIds returns empty set for empty input without I/O', async () => {
    const ids = await getTranscriptMediaIds([])
    expect(ids.size).toBe(0)
  })

  it('deleteTranscript removes the file', async () => {
    await saveTranscript(makeTranscript('m1'))
    await deleteTranscript('m1')
    expect(await getTranscript('m1')).toBeUndefined()
  })
})
