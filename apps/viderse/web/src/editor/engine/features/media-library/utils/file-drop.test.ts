// @vitest-environment node

import { describe, expect, it } from 'vitest'
import {
  extractValidMediaFileEntriesFromDataTransfer,
  formatMediaDropRejectionMessage,
} from './file-drop'

function makeItem(file: File | null, isDirectory = false): DataTransferItem {
  return {
    kind: 'file',
    getAsFile: () => file,
    webkitGetAsEntry: () => ({ isDirectory }) as FileSystemEntry,
  } as unknown as DataTransferItem
}

function makeDrop(...items: DataTransferItem[]): DataTransfer {
  return { items } as unknown as DataTransfer
}

describe('extractValidMediaFileEntriesFromDataTransfer', () => {
  it('reports dropped folders instead of silently ignoring them', async () => {
    const folder = new File([], 'Media Folder')

    const result = await extractValidMediaFileEntriesFromDataTransfer(
      makeDrop(makeItem(folder, true)),
    )

    expect(result.entries).toEqual([])
    expect(result.errors).toEqual([
      'Media Folder: folders are not supported yet. Drop media files directly.',
    ])
  })

  it('returns accepted files themselves, with no handle attached', async () => {
    const png = new File([new Uint8Array([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a])], 'a.png', {
      type: 'image/png',
    })

    const result = await extractValidMediaFileEntriesFromDataTransfer(makeDrop(makeItem(png)))

    expect(result.errors).toEqual([])
    expect(result.entries).toEqual([{ file: png, mediaType: 'image' }])
  })

  it('rejects unsupported files with a per-file message', async () => {
    const text = new File(['hello'], 'notes.txt', { type: 'text/plain' })

    const result = await extractValidMediaFileEntriesFromDataTransfer(makeDrop(makeItem(text)))

    expect(result.entries).toEqual([])
    expect(result.errors).toHaveLength(1)
    expect(result.errors[0]).toMatch(/^notes\.txt: /)
  })

  it('ignores non-file items and items without a file', async () => {
    const stringItem = { kind: 'string' } as unknown as DataTransferItem

    const result = await extractValidMediaFileEntriesFromDataTransfer(
      makeDrop(stringItem, makeItem(null)),
    )

    expect(result).toEqual({ entries: [], errors: [] })
  })
})

describe('formatMediaDropRejectionMessage', () => {
  it('summarizes rejected drops with examples and an overflow count', () => {
    expect(
      formatMediaDropRejectionMessage([
        'folder: folders are not supported yet. Drop media files directly.',
        'notes.txt: Unsupported file type',
        'archive.zip: Unsupported file type',
        'broken.mp4: Unable to read file',
      ]),
    ).toBe(
      '4 dropped items were rejected: folder: folders are not supported yet. Drop media files directly.; notes.txt: Unsupported file type; archive.zip: Unsupported file type; and 1 more.',
    )
  })
})
