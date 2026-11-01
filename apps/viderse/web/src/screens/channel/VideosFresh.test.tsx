// @vitest-environment happy-dom
// Kênh chưa có video (artboard Channel `fresh`): thẻ "Video mới nhất" lấy từ video đối thủ,
// thẻ "Tự lấy video" nói số đối thủ, nhịp và giờ Trinh sát xem gần nhất theo API quét.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { cache } from '@noverse/ui/data'
import type { Channel } from '../../api/channels'
import type { Rival } from '../../api/rivals'
import { VideosFresh } from './VideosFresh'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const source = (title: string, reference: boolean) => ({ rivalId: `UC${title}`, title, reference })
const channel = { id: 'c1', name: 'Kênh thử', autoPick: true, pacePerDay: 5, sources: [source('Nocturne Stories', true), source('Dark Hollow Tales', false)] } as unknown as Channel
const rival = { id: 'UCnocturne', title: 'Nocturne Stories', handle: '@nocturne' } as unknown as Rival

let root: Root
let host: HTMLDivElement
beforeEach(() => {
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  vi.stubGlobal('fetch', vi.fn((url: string) => Promise.resolve(Response.json(String(url).includes('/scan')
    ? { channelId: 'UCnocturne', discovery: { checkedAt: '2026-10-12T16:30:00Z', caughtUp: true, running: false, nextAt: null } }
    : { videos: [{ id: 'v1', title: 'The Lake House Guestbook Had One More Name', durationSec: 2920, publishedAt: new Date(Date.now() - 86_400_000).toISOString(), score: 2.7, thumbnailUrl: '', url: '' }], total: 1, nextCursor: null }))))
})
afterEach(() => { act(() => root.unmount()); host.remove(); vi.unstubAllGlobals(); cache.clear?.() })

const settle = () => act(async () => { await new Promise((resolve) => setTimeout(resolve, 30)) })

describe('VideosFresh', () => {
  it('thẻ đối thủ có tên video, dòng phụ "đối thủ · tuổi · ×điểm" và thời lượng', async () => {
    await act(async () => { root.render(<VideosFresh channel={channel} rival={rival} />) })
    await settle()
    expect(host.textContent).toContain('The Lake House Guestbook Had One More Name')
    expect(host.textContent).toContain('Nocturne Stories · 1 ngày · ×2,7')
    expect(host.textContent).toContain('48:40')
    expect(host.textContent).toContain('Làm video cho kênh')
  })

  it('thẻ Tự lấy video nói số đối thủ, nhịp và giờ xem gần nhất', async () => {
    await act(async () => { root.render(<VideosFresh channel={channel} rival={rival} />) })
    await settle()
    expect(host.textContent).toContain('Trinh sát xem 2 đối thủ mỗi giờ, đưa video mới đăng vào cuối hàng chờ.')
    expect(host.textContent).toMatch(/Nhịp 5\/ngày · xem gần nhất \d{2}:\d{2}/)
    expect(host.textContent).toContain('Đang bật')
    expect([...host.querySelectorAll('button')].some((b) => b.textContent === 'Đổi nhịp ›')).toBe(true)
  })

  it('kênh chưa liên kết đối thủ: tạm dừng và báo chưa có nguồn', async () => {
    const bare = { ...channel, autoPick: false, sources: [] } as unknown as Channel
    await act(async () => { root.render(<VideosFresh channel={bare} rival={undefined} />) })
    await settle()
    expect(host.textContent).toContain('Tạm dừng vì chưa có đối thủ liên kết')
    expect(host.textContent).toContain('Đang tắt')
  })
})
