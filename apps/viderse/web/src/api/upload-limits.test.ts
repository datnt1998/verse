import { afterEach, describe, expect, it, vi } from 'vitest'
import { fetchUploadLimits, resetUploadLimitsCache, uploadKindForMime, uploadTooLarge } from './upload-limits'

const hosted = {
  hosted: true,
  limits: { video: 90 << 20, audio: 90 << 20, image: 50 << 20, overlay: 90 << 20, template: 90 << 20 },
  message: 'Tải lên qua internet tối đa 90 MB',
}

const local = {
  hosted: false,
  limits: { video: 20 << 30, audio: 2 << 30, image: 50 << 20, overlay: 256 << 20, template: 4 << 30 },
  message: 'Tải lên qua internet tối đa 90 MB',
}

function stubFetch(payload: unknown) {
  const fetchMock = vi.fn(async () => new Response(JSON.stringify(payload), { status: 200, headers: { 'Content-Type': 'application/json' } }))
  vi.stubGlobal('fetch', fetchMock)
  return fetchMock
}

afterEach(() => {
  resetUploadLimitsCache()
  vi.unstubAllGlobals()
})

describe('uploadTooLarge', () => {
  it('nêu trần công khai khi tệp vượt giới hạn qua internet', async () => {
    stubFetch(hosted)
    await expect(uploadTooLarge('overlay', (91 << 20))).resolves.toBe('Tải lên qua internet tối đa 90 MB')
  })

  it('cho qua tệp nằm trong trần', async () => {
    stubFetch(hosted)
    await expect(uploadTooLarge('video', 90 << 20)).resolves.toBeNull()
  })

  it('không chặn gì khi chạy trên máy', async () => {
    stubFetch(local)
    await expect(uploadTooLarge('video', 5 << 30)).resolves.toBeNull()
  })

  it('không chặn khi không đọc được trần (offline)', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => { throw new Error('offline') }))
    await expect(uploadTooLarge('template', 200 << 20)).resolves.toBeNull()
  })

  it('đọc trần một lần cho mỗi phiên trang', async () => {
    const fetchMock = stubFetch(hosted)
    await fetchUploadLimits()
    await fetchUploadLimits()
    await uploadTooLarge('overlay', 1)
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })
})

describe('uploadKindForMime', () => {
  it('ánh xạ MIME sang loại trần', () => {
    expect(uploadKindForMime('video/mp4')).toBe('video')
    expect(uploadKindForMime('audio/wav')).toBe('audio')
    expect(uploadKindForMime('image/png')).toBe('image')
    expect(uploadKindForMime('')).toBe('image')
  })
})
