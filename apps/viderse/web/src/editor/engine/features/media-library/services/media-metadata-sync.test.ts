// @vitest-environment node

import { beforeEach, describe, expect, it, vi, type Mock } from 'vitest'
import type { MediaMetadata } from '@editor/types/storage'

const updateMedia = vi.hoisted(() => vi.fn<(id: string, update: unknown) => Promise<unknown>>())

vi.mock('@editor/features/media-library/deps/storage', () => ({ updateMedia }))

import {
  diffDurableMediaMetadata,
  startMediaMetadataSync,
  type MediaCommitEvent,
  type MediaCommitSource,
} from './media-metadata-sync'

function media(id: string, overrides: Partial<MediaMetadata> = {}): MediaMetadata {
  return {
    id,
    storageType: 'workspace',
    fileName: `${id}.mp4`,
    fileSize: 1,
    mimeType: 'video/mp4',
    duration: 1,
    width: 1,
    height: 1,
    fps: 30,
    codec: 'h264',
    bitrate: 0,
    tags: [],
    createdAt: 1,
    updatedAt: 1,
    ...overrides,
  }
}

function makeSource(): { source: MediaCommitSource; emit: (event: MediaCommitEvent) => void; off: Mock<() => void> } {
  let listener: ((event: MediaCommitEvent) => void) | null = null
  const off = vi.fn<() => void>(() => {
    listener = null
  })
  return {
    source: {
      on: (_event, next) => {
        listener = next
        return off
      },
    },
    emit: (event) => listener?.(event),
    off,
  }
}

function commit(
  commandType: string,
  before: MediaMetadata[],
  after: MediaMetadata[],
  kind: MediaCommitEvent['kind'] = 'execute',
): MediaCommitEvent {
  return {
    kind,
    commands: [{ type: commandType }],
    before: { mediaItems: before },
    after: { mediaItems: after },
  }
}

/** Let every already-queued promise continuation run (the sync queue is pure microtasks). */
async function flush(): Promise<void> {
  for (let turn = 0; turn < 50; turn++) await Promise.resolve()
}

beforeEach(() => {
  updateMedia.mockReset().mockResolvedValue(undefined)
})

describe('diffDurableMediaMetadata', () => {
  it('reports only the tags that changed', () => {
    expect(
      diffDurableMediaMetadata(
        [media('a', { tags: ['x'] }), media('b')],
        [media('a', { tags: ['x', 'y'] }), media('b')],
      ),
    ).toEqual([{ mediaId: 'a', tags: ['x', 'y'] }])
  })

  it('reports a removed attribution as null so the server clears it', () => {
    expect(
      diffDurableMediaMetadata(
        [media('a', { attribution: { provider: 'P' } })],
        [media('a')],
      ),
    ).toEqual([{ mediaId: 'a', attribution: null }])
  })

  it('treats equal attribution objects as unchanged', () => {
    expect(
      diffDurableMediaMetadata(
        [media('a', { attribution: { provider: 'P', license: 'L' } })],
        [media('a', { attribution: { provider: 'P', license: 'L' } })],
      ),
    ).toEqual([])
  })

  it('ignores media that were added or removed, and non-durable field changes', () => {
    expect(
      diffDurableMediaMetadata(
        [media('gone'), media('a')],
        [media('a', { fps: 60, duration: 9 }), media('new', { tags: ['t'] })],
      ),
    ).toEqual([])
  })

  it('returns nothing when either side has no media records', () => {
    expect(diffDurableMediaMetadata(undefined, [media('a')])).toEqual([])
    expect(diffDurableMediaMetadata([media('a')], undefined)).toEqual([])
  })
})

describe('startMediaMetadataSync', () => {
  it.each(['execute', 'undo', 'redo'] as const)('persists a tag change on %s', async (kind) => {
    const { source, emit } = makeSource()
    startMediaMetadataSync(source)

    emit(commit('media.update', [media('a')], [media('a', { tags: ['b-roll'] })], kind))
    await flush()

    expect(updateMedia).toHaveBeenCalledExactlyOnceWith('a', { tags: ['b-roll'] })
  })

  it('sends attribution null to clear and the new value to set', async () => {
    const { source, emit } = makeSource()
    startMediaMetadataSync(source)

    emit(commit('media.update', [media('a', { attribution: { provider: 'P' } })], [media('a')]))
    emit(commit('media.update', [media('b')], [media('b', { attribution: { provider: 'Q' } })]))
    await flush()

    expect(updateMedia).toHaveBeenCalledWith('a', { attribution: null })
    expect(updateMedia).toHaveBeenCalledWith('b', { attribution: { provider: 'Q' } })
  })

  it.each(['media.setRecords', 'media.add', 'media.removeRecords'])(
    'never writes back the records of a %s commit',
    async (commandType) => {
      const { source, emit } = makeSource()
      startMediaMetadataSync(source)

      emit(commit(commandType, [media('a')], [media('a', { tags: ['hydrated'] })]))
      await flush()

      expect(updateMedia).not.toHaveBeenCalled()
    },
  )

  it('still persists a batch that mixes an edit with other commands', async () => {
    const { source, emit } = makeSource()
    startMediaMetadataSync(source)

    emit({
      kind: 'execute',
      commands: [{ type: 'media.setRecords' }, { type: 'media.update' }],
      before: { mediaItems: [media('a')] },
      after: { mediaItems: [media('a', { tags: ['x'] })] },
    })
    await flush()

    expect(updateMedia).toHaveBeenCalledTimes(1)
  })

  it('applies two edits of one media in commit order even if the first is slower', async () => {
    const order: string[] = []
    let releaseFirst: () => void = () => undefined
    updateMedia.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          releaseFirst = () => {
            order.push('first')
            resolve(undefined)
          }
        }),
    )
    updateMedia.mockImplementationOnce(async () => {
      order.push('second')
    })
    const { source, emit } = makeSource()
    startMediaMetadataSync(source)

    emit(commit('media.update', [media('a')], [media('a', { tags: ['1'] })]))
    emit(commit('media.update', [media('a', { tags: ['1'] })], [media('a', { tags: ['2'] })]))
    await flush()
    expect(order).toEqual([])
    releaseFirst()
    await flush()

    expect(order).toEqual(['first', 'second'])
  })

  it('reports a failed write and keeps syncing later edits', async () => {
    const failure = new Error('server down')
    updateMedia.mockRejectedValueOnce(failure)
    const onFailure = vi.fn()
    const { source, emit } = makeSource()
    startMediaMetadataSync(source, onFailure)

    emit(commit('media.update', [media('a')], [media('a', { tags: ['1'] })]))
    await flush()
    emit(commit('media.update', [media('a', { tags: ['1'] })], [media('a', { tags: ['2'] })]))
    await flush()

    expect(onFailure).toHaveBeenCalledExactlyOnceWith('a', failure)
    expect(updateMedia).toHaveBeenCalledTimes(2)
  })

  it('returns the unsubscribe function of the source', () => {
    const { source, off } = makeSource()

    startMediaMetadataSync(source)()

    expect(off).toHaveBeenCalledTimes(1)
  })
})
