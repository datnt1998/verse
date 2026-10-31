import assert from 'node:assert/strict'
import { test } from 'node:test'
import { selectOverlayLibrary } from './overlay-library'

const record = (id: string, contentHash: string, createdAt: number, overlay?: boolean) => ({ id, contentHash, createdAt, ...(overlay ? { overlay } : {}) })

test('re-uploaded copies of one effect show once, as the oldest record, newest effect first', () => {
  const library = selectOverlayLibrary([
    record('sub-2', 'sha256:sub', 30, true),
    record('snow', 'sha256:snow', 10, true),
    record('sub-1', 'sha256:sub', 20, true),
    record('sub-3', 'sha256:sub', 40, true),
  ])
  assert.deepEqual(library.map((entry) => entry.id), ['sub-1', 'snow'])
})

test('media not tagged as an overlay stays out, even when it shares bytes with an overlay', () => {
  const library = selectOverlayLibrary([
    record('clip', 'sha256:clip', 5),
    record('sub-plain', 'sha256:sub', 1),
    record('sub', 'sha256:sub', 9, true),
  ])
  assert.deepEqual(library.map((entry) => entry.id), ['sub'])
})
