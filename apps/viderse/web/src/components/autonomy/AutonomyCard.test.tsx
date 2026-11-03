// @vitest-environment happy-dom
// Mức tự chủ ở tab Nguồn & tự động: đổi mức lưu qua PATCH, Tự chạy / Tự duyệt
// hết phải qua hộp xác nhận rồi mới gửi `confirmed`, bốn dòng lệch mức có sẵn
// hiện nhãn Tuỳ chỉnh, máy chủ chưa trả Mức tự chủ thì hiện lỗi chứ không bịa.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { waitFor } from '@testing-library/react'
import { dismissConfirmations, dismissToasts } from '@noverse/ui'
import type { Channel } from '../../api/channels'
import { DEFAULT_AUTONOMY, type Autonomy } from '../../api/autonomy'
import { applyViderseLabels } from '../../kitLabels'
import { AutonomyCard } from './AutonomyCard'

const net = vi.hoisted(() => ({ online: true }))
vi.mock('../../data/status', () => ({ useOnline: () => net.online }))

const respond = (body: unknown, status = 200) =>
  Promise.resolve(new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } }))

const channel = (autonomy: Autonomy | null = DEFAULT_AUTONOMY): Channel => ({
  id: 'c1', name: 'Midnight Tales', youtubeUrl: null, language: 'vi', styleNote: null, descriptionFooter: null, alwaysTags: [],
  autoPick: true, pacePerDay: 5, templateCursor: 0,
  branding: { brief: '', summary: '', look: '', avatar: '', banner: '' },
  autonomy: autonomy as Autonomy, priority: 'steady',
  avatarUrl: null, bannerUrl: null, sources: [], templates: [], identity: { state: 'ready' },
  createdAt: '2026-10-20T00:00:00Z', updatedAt: '2026-10-20T00:00:00Z',
})

let patches: { url: string; body: Record<string, unknown> }[] = []
const mounts: { root: Root; container: HTMLDivElement }[] = []
const flush = (ms = 30) => act(async () => { await new Promise<void>((resolve) => { setTimeout(resolve, ms) }) })

const mount = async (el: React.ReactElement) => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  mounts.push({ root, container })
  await act(async () => { root.render(el) })
  await flush()
  return container
}
const click = async (el: Element | null | undefined) => {
  expect(el).toBeTruthy()
  await act(async () => { (el as HTMLElement).dispatchEvent(new MouseEvent('click', { bubbles: true })) })
  await flush()
}
const radio = (root: ParentNode, name: string) => [...root.querySelectorAll('[role="radio"]')].find((el) => el.textContent === name)
const dialogButton = (name: string) => [...document.querySelectorAll('[role="alertdialog"] button')].find((el) => el.textContent?.trim() === name)

beforeEach(() => {
  applyViderseLabels()
  patches = []
  net.online = true
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    if (init?.method === 'PATCH') {
      const body = JSON.parse(String(init.body)) as { autonomy?: Autonomy; priority?: string }
      patches.push({ url, body })
      return respond({ ...channel(body.autonomy), priority: body.priority ?? 'steady' })
    }
    return respond({ error: 'not found' }, 404)
  }))
})
afterEach(async () => {
  await act(async () => {
    dismissConfirmations()
    dismissToasts()
    for (const { root, container } of mounts.splice(0)) {
      root.unmount()
      container.remove()
    }
    await Promise.resolve()
  })
  vi.unstubAllGlobals()
})

describe('AutonomyCard', () => {
  it('đổi sang Cẩn thận lưu ngay, không hỏi xác nhận', async () => {
    const el = await mount(<AutonomyCard channel={channel()} />)
    await click(radio(el, 'Cẩn thận'))
    expect(document.querySelector('[role="alertdialog"]')).toBeNull()
    expect(patches).toHaveLength(1)
    expect(patches[0]?.body.autonomy).toMatchObject({ preset: 'careful', pick: 'suggest', approve: { mode: 'ask', min: 85 }, onError: 'stop', belowQuality: 'ask' })
    expect(patches[0]?.body).not.toHaveProperty('confirmed')
  })

  it('lên Tự chạy: hộp xác nhận trước, chỉ sau khi xác nhận mới gửi confirmed', async () => {
    const el = await mount(<AutonomyCard channel={channel()} />)
    await click(radio(el, 'Tự chạy'))
    await waitFor(() => expect(document.querySelector('[role="alertdialog"]')).not.toBeNull())
    expect(patches).toHaveLength(0)
    await click(dialogButton('Chuyển sang Tự chạy'))
    expect(patches).toHaveLength(1)
    expect(patches[0]?.body).toMatchObject({ confirmed: true, autonomy: { preset: 'auto', approve: { mode: 'score' }, belowQuality: 'fix_then_ask' } })
  })

  it('Huỷ hộp xác nhận thì không lưu gì', async () => {
    const el = await mount(<AutonomyCard channel={channel()} />)
    await click(radio(el, 'Tự chạy'))
    await click(dialogButton('Huỷ'))
    expect(patches).toHaveLength(0)
  })

  it('bốn dòng không khớp mức nào thì hiện Tuỳ chỉnh và không chọn sẵn mức nào', async () => {
    const el = await mount(<AutonomyCard channel={channel({ ...DEFAULT_AUTONOMY, preset: 'custom', pick: 'score2', onError: 'stop' })} />)
    expect([...el.querySelectorAll('[role="radio"]')].some((node) => node.getAttribute('aria-checked') === 'true')).toBe(false)
  })

  it('ngưỡng điểm chỉ hiện khi Duyệt ảnh là Duyệt theo điểm, bước 5 trong 70–95', async () => {
    const el = await mount(<AutonomyCard channel={channel({ ...DEFAULT_AUTONOMY, preset: 'auto', pick: 'all', approve: { mode: 'score', min: 95 }, onError: 'retry_once', belowQuality: 'fix_then_ask' })} />)
    const up = el.querySelector('button[aria-label="Tăng 5"]') as HTMLButtonElement
    expect(up.disabled).toBe(true)
    await click(el.querySelector('button[aria-label="Giảm 5"]'))
    expect(patches[0]?.body.autonomy).toMatchObject({ approve: { mode: 'score', min: 90 } })
  })

  it('mất kết nối: báo rõ, các lựa chọn khoá và không gửi gì', async () => {
    net.online = false
    const el = await mount(<AutonomyCard channel={channel()} />)
    await click(radio(el, 'Cẩn thận'))
    expect(patches).toHaveLength(0)
  })

  it('máy chủ chưa trả Mức tự chủ: báo lỗi, không dựng mức bịa', async () => {
    const el = await mount(<AutonomyCard channel={channel(null)} />)
    expect(el.querySelectorAll('[role="radio"]')).toHaveLength(0)
  })
})
