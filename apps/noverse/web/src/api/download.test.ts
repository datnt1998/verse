// @vitest-environment happy-dom
import { afterEach, describe, expect, it, vi } from 'vitest'
import { downloadExportFile } from './client'

afterEach(() => vi.unstubAllGlobals())

describe('download filename handling', () => {
  if (!('createObjectURL' in URL)) Object.assign(URL, { createObjectURL: () => 'blob:x', revokeObjectURL: () => {} })
  it('decodes the RFC 5987 filename from the server', async () => {
    const disposition = `attachment; filename="chuong_mot.wav"; filename*=UTF-8''${encodeURIComponent('Chương một.wav')}`
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('x', { headers: { 'content-disposition': disposition } })))
    const done = await downloadExportFile('p', 's', 'a.wav')
    expect(done.file).toBe('Chương một.wav')
  })
  it('falls back to the ASCII filename when filename* is malformed', async () => {
    // Truncated 3-byte 'ế' — the defect the backend sanitizer used to emit.
    const disposition = `attachment; filename="chuong_mot.wav"; filename*=UTF-8''%E1%BA`
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('x', { headers: { 'content-disposition': disposition } })))
    const done = await downloadExportFile('p', 's', 'a.wav')
    expect(done.file).toBe('chuong_mot.wav')
  })
  it('uses the fallback name when disposition has neither form', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('x', { headers: { 'content-disposition': 'attachment' } })))
    const done = await downloadExportFile('p', 's', 'a.wav')
    expect(done.file).toBe('a.wav')
  })
})
