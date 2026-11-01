// Chỉ chạy khi dev (`import.meta.env.DEV` + `#/review?fixture=1`): chặn các
// endpoint ảnh của màn Duyệt bằng dữ liệu mẫu §26 để xem giao diện trước khi
// máy chủ có ảnh thật. Bản build không nạp tệp này (nhánh DEV bị loại bỏ).
import type { ImageProgress } from '../api/images'

const plate = (hue: number, text: string) => {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="1280" height="720"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="hsl(${hue} 42% 26%)"/><stop offset="1" stop-color="hsl(${hue + 30} 38% 12%)"/></linearGradient></defs><rect width="1280" height="720" fill="url(#g)"/><circle cx="980" cy="180" r="120" fill="hsl(${hue + 60} 55% 62% / .35)"/><rect x="120" y="520" width="1040" height="8" rx="4" fill="hsl(${hue + 60} 40% 70% / .3)"/><text x="120" y="420" font-family="system-ui" font-size="54" fill="hsl(${hue} 30% 92%)">${text}</text></svg>`
  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`
}

const part = (kind: string, text: string, type: string, label: string) => ({ kind, text, source: { type, label } })
const ref = (id: string, role: string, hue: number, label: string, atSec: number) => ({
  id, role, url: plate(hue, 'ref'), source: { type: 'rival_frame', label, videoId: 'rival-1', atSec },
})

const version = (n: number, hue: number, extra: Record<string, unknown> = {}) => ({
  n,
  state: 'ready',
  url: plate(hue, `Bản ${n}`),
  previewUrl: null,
  width: 1280,
  height: 720,
  parts: [
    part('scene', n === 1
      ? 'A night guard stands at the bottom of a stopped escalator, shopping mall lights off, cold blue haze.'
      : 'A night guard stands on the roof of the mall, a ladder leading up from the fire escape, cold blue haze, rain on the railing.', n === 1 ? 'artist' : 'artist_feedback', n === 1 ? 'Hoạ sĩ viết từ lời kể' : 'Hoạ sĩ sửa theo góp ý'),
    part('style', 'Flat cinematic 2D, teal and rust palette, soft rim light, 35mm grain.', 'image_prompt', 'prompt “Đêm xanh rêu”'),
    part('refs', 'Image 1: keep the camera angle and the ladder position. Image 2: the guard’s coat and face.', 'artist', '2 ảnh bên dưới'),
    part('learned', 'Never add readable text except shop signs. Keep the mall empty of shoppers.', 'image_prompt', 'prompt “Đêm xanh rêu”'),
    part('avoid', 'blurry, watermark, extra fingers, warped perspective', 'template', 'template Mẫu chuẩn'),
  ],
  sentText: 'A night guard stands at the bottom of a stopped escalator, shopping mall lights off, cold blue haze.\n\nFlat cinematic 2D, teal and rust palette, soft rim light, 35mm grain.\n\nImage 1: keep the camera angle and the ladder position. Image 2: the guard’s coat and face.\n\nNever add readable text except shop signs. Keep the mall empty of shoppers.\n\nAvoid: blurry, watermark, extra fingers, warped perspective',
  refs: [ref(`r${n}-1`, 'layout', 200, 'Khung đối thủ · Nocturne Stories 14:02', 842), ref(`r${n}-2`, 'character', 28, 'Ảnh cảnh 1 · áo và mặt bảo vệ', 0)],
  provider: 'chatgpt',
  model: 'gpt-image-2',
  params: { size: '1280x720', quality: 'high' },
  feedback: n === 1 ? null : 'bảo vệ đứng trên đầu thang, tông lạnh hơn',
  diffFromPrev: n === 1 ? null : [['A night guard stands', ''], [' at the bottom of a stopped escalator', 'del'], [' on the roof of the mall', 'ins'], [', a ladder leading up from the fire escape', 'ins']],
  startedAt: '2026-11-01T16:58:00Z',
  finishedAt: '2026-11-01T16:58:44Z',
  durationSec: 44,
  costImages: 1,
  error: null,
  ...extra,
})

const scene = (index: number, hue: number, extra: Record<string, unknown> = {}) => ({
  id: `img-${index}`,
  kind: 'scene',
  index,
  label: `Cảnh ${index}`,
  usedVersion: 1,
  needsReview: false,
  status: 'review',
  refLimit: 4,
  versions: [version(1, hue)],
  narration: `Lời kể cảnh ${index}: người bảo vệ đi qua sảnh trung tâm thương mại đã đóng cửa, đèn hành lang nhấp nháy, tiếng mưa ngoài cửa kính.`,
  startSec: 720 + index * 60,
  endSec: 780 + index * 60,
  direction: 'Máy quay ngang, chậm, tông lạnh',
  ...extra,
})

const images = [
  scene(1, 205),
  scene(2, 190, {
    usedVersion: 2,
    versions: [version(1, 190), version(2, 196)],
  }),
  scene(3, 175, {
    usedVersion: 2,
    versions: [version(1, 175), version(2, 180), version(3, 185, { state: 'drawing', url: null, previewUrl: null, finishedAt: null, durationSec: null, costImages: 0, feedback: 'bảo vệ đứng trên đầu thang' })],
  }),
  scene(4, 250, {
    usedVersion: 1,
    versions: [version(1, 250), version(2, 255, {
      state: 'failed', url: null, finishedAt: null, durationSec: null, costImages: 0,
      error: { reason: 'quota', retryAt: '2026-11-02T00:30:00Z', message: 'ChatGPT đã hết hạn mức ảnh hôm nay (50/50)' },
      parts: [part('scene', 'A night guard on the roof, ladder up from the fire escape, colder grade.', 'artist_feedback', 'Hoạ sĩ sửa theo góp ý'), part('style', 'Flat cinematic 2D, teal and rust palette.', 'image_prompt', 'prompt “Đêm xanh rêu”')],
    })],
  }),
  scene(5, 160),
  scene(6, 145),
  scene(7, 130),
  { id: 'cover', kind: 'cover', index: 8, label: 'Ảnh bìa', usedVersion: 1, needsReview: false, status: 'approved', refLimit: 4, versions: [version(1, 30)], narration: '' },
]

const quota = {
  providers: [{ provider: 'chatgpt', label: 'ChatGPT', left: 7, total: 50, resetAt: '2026-11-02T00:30:00Z', paid: true }],
  left: 7,
  fallback: { provider: 'grok', label: 'Grok', left: 190 },
}

const jobs = [{ id: 'fixture-job', channelId: 'fixture-channel', stageGroup: 'review', subline: '4 cảnh · 7 ảnh · 12:00', createdAt: '2026-11-01T10:00:00Z', source: { title: 'Mall closes at 10PM', thumbnailUrl: null } }]

const legacyItem = (image: (typeof images)[number]) => ({
  id: image.id,
  kind: image.kind,
  status: image.status,
  prompt: image.versions[0].parts[0].text,
  currentVersion: image.usedVersion,
  reviewVersion: image.usedVersion,
  versions: image.versions.map((entry) => ({ version: entry.n, cutout: null, originalUrl: entry.url ?? undefined })),
  sceneId: image.kind === 'scene' ? `scene-${image.index}` : undefined,
  startSec: 'startSec' in image ? image.startSec : undefined,
  endSec: 'endSec' in image ? image.endSec : undefined,
  width: 1280,
  height: 720,
  narration: 'narration' in image ? image.narration : undefined,
  direction: 'direction' in image ? image.direction : undefined,
  referenceUrl: plate(200, 'ref'),
  reference: 'Khung đối thủ · Nocturne Stories 14:02',
})

const json = (body: unknown, status = 200) => new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } })
const progress = (imageId: string, n: number, phase: ImageProgress['phase'], elapsedSec: number): ImageProgress => ({ jobId: 'fixture-job', imageId, n, phase, elapsedSec })

/** Giả lập EventSource để có tiến độ thật (đang vẽ → xong) như máy chủ gửi. */
class FixtureEvents extends EventTarget {
  static readonly CONNECTING = 0
  static readonly OPEN = 1
  static readonly CLOSED = 2
  readonly url: string
  readonly withCredentials = true
  readyState = 1
  onmessage: ((event: MessageEvent) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  onopen: ((event: Event) => void) | null = null
  constructor(url: string | URL) {
    super()
    this.url = String(url)
    window.setTimeout(() => {
      this.onopen?.(new Event('open'))
      this.dispatchEvent(new Event('open'))
    }, 60)
    let elapsed = 18
    const tick = window.setInterval(() => {
      elapsed += 6
      const event = new MessageEvent('image.progress', { data: JSON.stringify(progress('img-3', 3, elapsed > 36 ? 'receiving' : 'drawing', elapsed)) })
      this.dispatchEvent(event)
      this.onmessage?.(event)
      if (elapsed > 48) window.clearInterval(tick)
    }, 3000)
  }
  addEventListener(type: string, listener: EventListenerOrEventListenerObject | null, options?: boolean | AddEventListenerOptions) { super.addEventListener(type, listener, options) }
  removeEventListener(type: string, listener: EventListenerOrEventListenerObject | null) { super.removeEventListener(type, listener) }
  close() { this.readyState = 2 }
  dispatchEvent(event: Event) { return super.dispatchEvent(event) }
}

const real = globalThis.fetch
globalThis.fetch = async (input: RequestInfo | URL, init?: RequestInit) => {
  const url = String(input instanceof Request ? input.url : input)
  const method = (init?.method ?? (input instanceof Request ? input.method : 'GET')).toUpperCase()
  if (url.includes('/api/clone?channel=all')) return json({ jobs })
  if (url.endsWith('/api/clone/fixture-job/images') && method === 'GET') return json({ images, items: images.map(legacyItem), quota, manifest: { revision: 7 } })
  if (url.includes('/images/redraw')) return json({ versions: [{ imageId: 'img-3', n: 4 }] }, 202)
  if (url.includes('/use')) return json({ imageId: 'img-2', n: 2, usedVersion: 2, needsReview: true })
  if (url.includes('/cancel')) return json({ imageId: 'img-3', n: 3, cancelled: true, costImages: 0, usedVersion: 2 })
  if (url.includes('/api/clone/counts')) return json({ approvedToday: 3 })
  if (url.includes('/api/me')) return json({ user: { username: 'fixture', name: 'Fixture', role: 'admin', grants: { viderse: 'admin' } }, session: { expiresAt: '2099-01-01T00:00:00Z' }, apps: [] })
  if (url.includes('/api/setup')) return json({ needed: false })
  if (url.includes('/api/status')) return json({ running: 0, queued: 0, paused: false, reviewCount: 1, attention: [] })
  if (url.includes('/api/owner-channels')) return json({ channels: [{ id: 'fixture-channel', name: 'Nocturne Stories', competitors: [], autonomy: null, createdAt: '2026-10-01T00:00:00Z' }] })
  if (url.includes('/api/activity') || url.includes('/api/suggestions') || url.includes('/api/notifications')) return json({})
  // Mọi endpoint khác: trả rỗng để khung không hiện băng "mất kết nối" khi chụp.
  if (url.includes('/api/') && method === 'GET') return json({})
  return real(input as RequestInfo, init)
}
Object.defineProperty(globalThis, 'EventSource', { value: FixtureEvents, configurable: true })
;(globalThis as { __vvFixture?: boolean }).__vvFixture = true
