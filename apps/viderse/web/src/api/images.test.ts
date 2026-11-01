// Hợp đồng §26 — mẫu đúng payload máy chủ trả (một ảnh, ba bản) và luật ghép
// tiến độ SSE vào danh sách ảnh đang hiện.
import { describe, expect, it } from 'vitest'
import {
  cancelImageVersion, getJobImages, imageProgressSchema, imageVersionSchema, jobImagesSchema, redrawImages,
  subscribeImageProgress, tooManyRefs, useImageVersion, withProgress, type JobImage,
} from './images'

const version = (n: number, state: 'drawing' | 'ready' | 'failed' = 'ready') => ({
  n, state, url: state === 'ready' ? `/api/clone/j/images/i/versions/${n}` : null, previewUrl: null,
  width: 1280, height: 720,
  parts: [
    { kind: 'scene', text: 'A guard stands on top of a fire escape at night.', source: { type: 'artist', label: 'Hoạ sĩ viết từ lời kể' } },
    { kind: 'refs', text: 'Image 1: layout from the rival frame. Image 2: the guard’s appearance.', source: { type: 'artist', label: '2 ảnh bên dưới' } },
  ],
  sentText: 'A guard stands on top of a fire escape at night.\n\nImage 1: layout from the rival frame. Image 2: the guard’s appearance.',
  refs: [{ id: 'r1', role: 'layout', url: '/blob/r1.jpg', source: { type: 'rival_frame', label: 'Khung đối thủ · Nocturne Stories 14:02', videoId: 'v', atSec: 842 } }],
  provider: 'chatgpt', model: 'gpt-image-2', params: { size: '1280x720' },
  feedback: n === 1 ? null : 'bảo vệ đứng trên đầu thang, tông lạnh hơn',
  diffFromPrev: n === 1 ? null : [['A guard', ''], [' on the roof', 'del'], ['a ladder', 'ins']],
  startedAt: '2026-11-01T16:58:00Z', finishedAt: state === 'drawing' ? null : '2026-11-01T16:58:44Z',
  durationSec: state === 'drawing' ? null : 44, costImages: 1,
  error: state === 'failed' ? { reason: 'quota', retryAt: '2026-11-02T00:00:00Z', message: 'ChatGPT đã hết hạn mức ảnh hôm nay (50/50)' } : null,
})

const payload = {
  images: [{
    id: 'img3', kind: 'scene', index: 2, label: 'Cảnh 3', usedVersion: 2, refLimit: 4,
    versions: [version(1), version(2), version(3, 'drawing')],
    narration: 'Lời kể của cảnh 3.', startSec: 745, endSec: 1120,
  }],
}

describe('hợp đồng §26', () => {
  it('đọc đúng payload { images } gồm phần prompt, refs, diff và meta', () => {
    const parsed = jobImagesSchema.parse(payload)
    const image = parsed.images[0]
    expect(image.usedVersion).toBe(2)
    expect(image.refLimit).toBe(4)
    expect(image.versions.map((v) => v.n)).toEqual([1, 2, 3])
    expect(image.versions[1].parts.map((part) => part.kind)).toEqual(['scene', 'refs'])
    expect(image.versions[1].diffFromPrev).toEqual([['A guard', ''], [' on the roof', 'del'], ['a ladder', 'ins']])
    expect(image.versions[1].refs[0].source.atSec).toBe(842)
    // trường ngoài §26 vẫn đi qua cho màn Duyệt (lời kể, khung giờ)
    expect((image as JobImage & { narration?: string }).narration).toBe('Lời kể của cảnh 3.')
  })

  it('bỏ qua items (hình dạng cũ) mà không làm hỏng cả payload', () => {
    const parsed = jobImagesSchema.parse({ ...payload, items: [{ id: 'a', kind: 'scene', status: 'review', prompt: 'p', versions: [] }] })
    expect(parsed.images).toHaveLength(1)
    expect(jobImagesSchema.parse({ items: [{ id: 'a' }] }).images).toHaveLength(0)
  })

  it('bản lỗi giữ lý do thật và giờ đặt lại', () => {
    const failed = imageVersionSchema.parse(version(3, 'failed'))
    expect(failed.error?.reason).toBe('quota')
    expect(failed.error?.retryAt).toBe('2026-11-02T00:00:00Z')
  })

  it('tiến độ SSE đúng tên trường', () => {
    expect(imageProgressSchema.parse({ jobId: 'j', imageId: 'img3', n: 3, phase: 'receiving', previewUrl: '/p.jpg', elapsedSec: 36 }).phase).toBe('receiving')
    expect(imageProgressSchema.safeParse({ jobId: 'j', imageId: 'img3', n: 3, phase: 'nope', elapsedSec: 1 }).success).toBe(false)
  })
})

describe('ghép tiến độ vào ảnh', () => {
  const image = jobImagesSchema.parse(payload).images[0]

  it('bản đang vẽ nhận previewUrl từ SSE', () => {
    const next = withProgress(image, { jobId: 'j', imageId: 'img3', n: 3, phase: 'receiving', previewUrl: '/preview.jpg', elapsedSec: 36 })
    const drawing = next.versions.find((v) => v.n === 3)!
    expect(drawing.state).toBe('drawing')
    expect(drawing.previewUrl).toBe('/preview.jpg')
    expect(next.versions.find((v) => v.n === 2)!.previewUrl).toBeNull()
  })

  it('server chưa trả phiên bản mới thì vẫn hiện bản đang vẽ', () => {
    const next = withProgress(image, { jobId: 'j', imageId: 'img3', n: 4, phase: 'drawing', elapsedSec: 4 })
    expect(next.versions.at(-1)).toMatchObject({ n: 4, state: 'drawing', costImages: 0 })
    expect(next.usedVersion).toBe(2)
  })

  it('tiến độ của ảnh khác không đụng vào ảnh này', () => {
    expect(withProgress(image, { jobId: 'j', imageId: 'img4', n: 3, phase: 'drawing', elapsedSec: 1 })).toBe(image)
  })
})

describe('gọi máy chủ', () => {
  it('đường dẫn đúng §26 và lỗi quá ảnh tham chiếu đọc được limit', async () => {
    const calls: Array<[string, RequestInit]> = []
    const original = globalThis.fetch
    globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
      calls.push([String(input), init ?? {}])
      const path = String(input)
      if (path.endsWith('/redraw')) return new Response(JSON.stringify({ error: 'Chỉ nhận tối đa 4 ảnh tham chiếu.', code: 'too_many_refs', limit: 4 }), { status: 422 })
      if (path.endsWith('/images')) return new Response(JSON.stringify(payload))
      if (path.endsWith('/cancel')) return new Response(JSON.stringify({ imageId: 'img3', n: 3, cancelled: true, costImages: 0, usedVersion: 2 }))
      if (path.endsWith('/use')) return new Response(JSON.stringify({ imageId: 'img3', n: 2, usedVersion: 2, needsReview: true, clipId: 'c1', src: '/api/clone/j/images/img3?v=2' }))
      return new Response(JSON.stringify({ ok: true }))
    }) as typeof fetch
    try {
      expect((await getJobImages('j')).images).toHaveLength(1)
      const error = await redrawImages('j', { imageIds: ['img3'], parts: { scene: 'A new scene.' } }).catch((err: unknown) => err)
      expect(tooManyRefs(error)).toBe(4)
      const used = await useImageVersion('j', 'img3', 2, { project: false })
      expect(used).toMatchObject({ usedVersion: 2, needsReview: true, clipId: 'c1' })
      expect(await cancelImageVersion('j', 'img3', 3)).toMatchObject({ cancelled: true, costImages: 0, usedVersion: 2 })
      expect(calls.map(([path]) => path)).toEqual([
        '/api/clone/j/images',
        '/api/clone/j/images/redraw',
        '/api/clone/j/images/img3/use',
        '/api/clone/j/images/img3/versions/3/cancel',
      ])
      expect(JSON.parse(String(calls[2][1].body))).toEqual({ n: 2, project: false })
    } finally {
      globalThis.fetch = original
    }
  })

  it('retains history and reference URLs when the artist quota is unavailable', async () => {
    const original = globalThis.fetch
    const refs = [
      { id: 'upload:media-1', role: 'character', url: '/api/clone/j/artifacts/upload-ref', source: { type: 'upload', label: 'Bạn tải lên', imageId: 'media-1' } },
      { id: 'rival:v:1.25', role: 'layout', url: '/api/clone/j/artifacts/frame-ref', source: { type: 'rival_frame', label: 'Khung đối thủ', videoId: 'v', atSec: 1.25 } },
    ]
    const history = { ...payload, quota: null, images: [{ ...payload.images[0], versions: [{ ...version(1), refs }] }] }
    globalThis.fetch = (async () => new Response(JSON.stringify(history))) as typeof fetch
    try {
      const parsed = await getJobImages('j')
      expect(parsed.quota).toBeUndefined()
      expect(parsed.images[0].versions[0].refs).toEqual(refs)
      expect(parsed.images[0].versions[0].url).toBe(version(1).url)
    } finally {
      globalThis.fetch = original
    }
  })

  it('rejects missing or empty reference URLs instead of hiding missing artifacts', () => {
    const ref = version(1).refs[0]
    for (const invalid of [{ ...ref, url: '' }, { id: ref.id, role: ref.role, source: ref.source }]) {
      expect(imageVersionSchema.safeParse({ ...version(1), refs: [invalid] }).success).toBe(false)
    }
  })

  it('đăng ký sự kiện image.progress trên luồng chung', () => {
    expect(typeof subscribeImageProgress).toBe('function')
  })
})
