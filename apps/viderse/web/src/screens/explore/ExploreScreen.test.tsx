// @vitest-environment happy-dom
// Khám phá — hành vi màn: lưới ảo hoá giới hạn số thẻ DOM cho 20.000 video và
// nạp trang tiếp theo khi gần đáy; chọn nhiều (Top 10/20 · Bỏ chọn · lựa chọn
// sống qua refetch SSE `rivals`); điều khiển khoá hiện lý do; bấm thẻ → panel
// xem trước. Dữ liệu qua fetch giả; SSE mô phỏng bằng setData vào đúng khoá
// cache của trang đầu (đây chính là đường startDataEvents invalidate đi).
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { invalidate, setData } from '@noverse/ui/data'
import { ExploreScreen } from './ExploreScreen'
import { useVirtualGrid } from '../../lib/useVirtualGrid'
import { useRivalVideos, type RivalVideos } from './useRivalVideos'
import { VideoGrid } from './VideoGrid'
import { keys } from '../../data/keys'
import { vi as t } from '../../i18n/vi'
import type { PostingSchedule, Rival, RivalList, ScanStatus, Video, VideoPage } from '../../api/rivals'

const tt = t.screens.explore
const ROUTE = { name: 'explore' as const }

const video = (i: number, patch: Partial<Video> = {}): Video => ({
  id: `v${i}`, channelId: 'r1', title: `Video số ${i}`, durationSec: 2391,
  viewCount: 1_200_000, publishedAt: '2026-10-28T04:00:00Z', exactTime: true,
  score: i === 0 ? 4.2 : 2.1, hasCaptions: i % 3 === 0 ? true : null,
  scoreReason: i === 0 ? { agent: 'scout', text: 'Tổng lượt xem gấp 4,2 lần trung vị.', facts: [{ label: 'Trung vị kênh', value: '285 N', source: '150 video gần nhất' }] } : undefined,
  thumbnailUrl: `https://i.ytimg.com/vi/v${i}/mqdefault.jpg`, url: `https://youtu.be/v${i}`,
  ...patch,
})

const rival: Rival = {
  id: 'r1', handle: '@motphim', title: 'Một Phim', avatarUrl: null,
  url: 'https://youtube.com/@motphim', subscriberCount: 1_240_000,
  videoCount: 1284, medianViews: 286_000, outliers: 42, newOutliers: 4,
  addedAt: '2026-10-20T00:00:00Z', scannedAt: '2026-10-29T08:00:00Z',
  scan: { state: 'idle', discoveryRunning: false },
}

const scanStatus: ScanStatus = {
  channelId: 'r1', historyCap: 2000, indexedVideos: 1284, rescanHours: 6,
  history: { state: 'idle', jobId: null, scanned: 0, added: 0, pages: 0, reachedEnd: false, startedAt: null, updatedAt: null, nextAttemptAt: null, errorCode: null, error: null, recovery: null },
  discovery: { checkedAt: null, caughtUp: true, running: false, nextAt: null },
}

const posting: PostingSchedule = {
  channelId: 'r1', zone: 'vn', sample: 30, total: 30, pending: 0, pausedUntil: null,
  perWeek: 2.3, weeksTotal: 6, offSlot: 12,
  grid: Array.from({ length: 7 }, (_, d) => Array.from({ length: 12 }, (_, b) => (d === 3 && b === 10 ? 8 : d === 5 && b === 10 ? 6 : 1))),
  slots: [
    { weekdays: [3], start: '20:00', end: '22:00', count: 14, weeksHit: 6, weeksTotal: 6, sinceWeeks: 0, rhythm: 'weekly', isTop: true },
    { weekdays: [5], start: '20:00', end: '22:00', count: 10, weeksHit: 4, weeksTotal: 6, sinceWeeks: 0, rhythm: 'most_weeks', isTop: false },
  ],
}

let videos: Video[] = []
let nextCursor: string | null = null
let listBody: RivalList = { rivals: [rival], totalVideos: 1284, newOutliers: 4 }
let posted: { url: string; body: unknown }[] = []
/** các giá trị q mà lưới đã hỏi API (đếm số lần gọi khi gõ) */
let searched: string[] = []
/** số lần trang đầu của lưới phải trả lỗi trước khi thành công (nút Thử lại) */
let videosFails = 0

const respond = (body: unknown, status = 200) =>
  Promise.resolve(new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } }))

function stubFetch() {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    if (url === '/api/rivals') {
      if (init?.method === 'POST') { posted.push({ url, body: JSON.parse(String(init.body)) }); return respond(rival, 201) }
      return respond(listBody)
    }
    if (url.startsWith('/api/rivals/all/videos')) {
      if (videosFails > 0) { videosFails -= 1; return respond({ error: 'boom' }, 500) }
      const params = new URL(url, 'https://x.test').searchParams
      const cursor = params.get('cursor')
      const q = params.get('q')
      if (q !== null) searched.push(q)
      // Server tìm không phân biệt hoa thường/dấu; fetch giả chỉ cần lọc theo chữ thường.
      const pool = q ? videos.filter((v) => v.title.toLowerCase().includes(q.toLowerCase())) : videos
      if (cursor === 'c2') return respond({ videos: pool.slice(200, 400), total: pool.length, nextCursor: 'c3' } satisfies VideoPage)
      return respond({ videos: pool.slice(0, 200), total: pool.length, nextCursor: pool.length > 200 ? nextCursor : null } satisfies VideoPage)
    }
    if (url === '/api/rivals/r1/scan') return respond(scanStatus)
    if (url.startsWith('/api/rivals/r1/posting-slots')) return respond(posting)
    return respond({ error: 'not found' }, 404)
  }))
}

const roots: Root[] = []
const flush = async (ms = 30) => { await act(async () => { await new Promise((r) => setTimeout(r, ms)) }) }

const mount = async (el: React.ReactElement) => {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(el) })
  await flush()
  return container
}

const click = async (el: Element | null | undefined) => {
  expect(el).toBeTruthy()
  await act(async () => { (el as HTMLElement).dispatchEvent(new MouseEvent('click', { bubbles: true })) })
  await flush()
}

const byText = (root: ParentNode, sel: string, text: string) =>
  [...root.querySelectorAll(sel)].find((el) => el.textContent?.includes(text))

/** Chờ điều kiện có thật (fetch giải quyết bất đồng bộ qua microtask + timer). */
const until = async (fn: () => boolean, tries = 40) => {
  for (let i = 0; i < tries; i++) {
    if (fn()) return
    await flush()
  }
  expect(fn()).toBe(true)
}

/** Tooltip của kit hiện sau 0,4 s hover — dispatch pointerenter + chờ timer. */
const showTip = async (el: Element) => {
  await act(async () => {
    el.dispatchEvent(new PointerEvent('pointerover', { bubbles: true }))
    el.dispatchEvent(new PointerEvent('pointerenter', { bubbles: false }))
    el.dispatchEvent(new FocusEvent('focusin', { bubbles: true }))
  })
  await act(async () => { await new Promise((r) => setTimeout(r, 480)) })
  return [...document.body.querySelectorAll('[role="tooltip"]')].pop()?.textContent ?? ''
}

/** Điều khiển cần kênh của bạn: khoá bằng aria-disabled (không phải disabled), vẫn
 *  focus được, aria-describedby trỏ tới một phần tử có đúng câu lý do. */
const expectLocked = (el: HTMLElement) => {
  expect(el.getAttribute('aria-disabled')).toBe('true')
  expect((el as HTMLButtonElement).disabled ?? false).toBe(false)
  el.focus()
  expect(document.activeElement).toBe(el)
  const reasons = (el.getAttribute('aria-describedby') ?? '').split(/\s+/).filter(Boolean)
    .map((id) => document.getElementById(id)?.textContent)
  expect(reasons).toContain(tt.lockedReason)
}

/** Gõ vào ô (setter gốc + sự kiện input để React nhận). */
const typeIn = async (input: HTMLInputElement, value: string) => {
  await act(async () => {
    Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')?.set?.call(input, value)
    input.dispatchEvent(new Event('input', { bubbles: true }))
  })
}

const press = async (target: EventTarget, key: string) => {
  let event: KeyboardEvent | undefined
  await act(async () => {
    event = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true })
    target.dispatchEvent(event)
  })
  return event!
}


beforeEach(() => {
  videos = Array.from({ length: 240 }, (_, i) => video(i))
  nextCursor = 'c2'
  posted = []
  searched = []
  videosFails = 0
  listBody = { rivals: [rival], totalVideos: 1284, newOutliers: 4 }
  void invalidate()
  vi.stubGlobal('matchMedia', vi.fn().mockImplementation(() => ({
    matches: false, addEventListener: () => {}, removeEventListener: () => {},
  })))
  stubFetch()
})

afterEach(async () => {
  for (const root of roots.splice(0)) await act(async () => { root.unmount() })
  document.body.innerHTML = ''
  vi.unstubAllGlobals()
})

describe('lưới video ảo hoá', () => {
  it('20.000 video chỉ mount ≤ 200 thẻ và cửa sổ trượt theo cuộn', async () => {
    const big = Array.from({ length: 20_000 }, (_, i) => video(i))
    const Feed = () => {
      const grid = useVirtualGrid({ count: big.length, viewport: { width: 980, height: 880 } })
      return (
        <div ref={grid.ref} onScroll={grid.onScroll} style={{ height: 880, overflowY: 'auto' }}>
          <VideoGrid items={big} cols={grid.cols} start={grid.itemStart} end={grid.itemEnd}
            padTop={grid.padTop} padBottom={grid.padBottom} pickedId={null} selectMode={false}
            checkedIds={new Set()} onOpen={() => {}} onToggle={() => {}} hasChannel={false} />
        </div>
      )
    }
    const el = await mount(<Feed />)
    const count = () => el.querySelectorAll('.vx-card').length
    expect(count()).toBeGreaterThan(0)
    expect(count()).toBeLessThanOrEqual(200)

    const scroller = el.firstElementChild as HTMLElement
    scroller.scrollTop = 500_000
    await act(async () => { scroller.dispatchEvent(new Event('scroll', { bubbles: true })) })
    await flush()
    const titles = [...el.querySelectorAll('.vx-vtitle')].map((e) => e.textContent)
    expect(titles.some((s) => s && /Video số (7[0-9]{3}|8[0-9]{3}|9[0-9]{3})/.test(s))).toBe(true)
    expect(count()).toBeLessThanOrEqual(200)
  })

  it('điện thoại (390×844): một cột, cửa sổ vẫn bị chặn ≤ 200 thẻ', async () => {
    const big = Array.from({ length: 20_000 }, (_, i) => video(i))
    const Feed = () => {
      const grid = useVirtualGrid({ count: big.length, viewport: { width: 390, height: 844 } })
      return (
        <div ref={grid.ref} onScroll={grid.onScroll} style={{ height: 844, overflowY: 'auto' }}>
          <VideoGrid items={big} cols={grid.cols} start={grid.itemStart} end={grid.itemEnd}
            padTop={grid.padTop} padBottom={grid.padBottom} pickedId={null} selectMode={false}
            checkedIds={new Set()} onOpen={() => {}} onToggle={() => {}} hasChannel={false} />
        </div>
      )
    }
    const el = await mount(<Feed />)
    expect(el.querySelectorAll('.vx-card').length).toBeLessThanOrEqual(200)
    expect(el.querySelectorAll('.vx-card').length).toBeGreaterThan(0)
    // 390px chỉ đủ một cột → mỗi hàng một thẻ (không tràn ngang do grid 1 cột)
    const row = el.querySelector<HTMLElement>('.vx-row')!
    expect(row.style.getPropertyValue('--cols')).toBe('1')
  })

  it('gần đáy danh sách đã tải → nạp trang tiếp (cursor)', async () => {
    const page1 = { videos: videos.slice(0, 200), total: 1000, nextCursor: 'c2' }
    const page2 = { videos: videos.slice(200, 240), total: 1000, nextCursor: null }
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
      if (url.includes('/api/rivals/all/videos')) {
        return respond(new URL(url, 'https://x.test').searchParams.get('cursor') === 'c2' ? page2 : page1)
      }
      return respond({ error: 'not found' }, 404)
    }))
    let hook: RivalVideos | undefined
    const Probe = () => {
      hook = useRivalVideos({ scope: 'all', sort: 'outlier', long: false, within: 30, q: '' })
      return null
    }
    await mount(<Probe />)
    await until(() => hook!.items.length === 200)
    expect(hook!.hasMore).toBe(true)
    await act(async () => { hook!.needMore() })
    await until(() => hook!.items.length === 240)
    expect(hook!.hasMore).toBe(false)
  })
})

describe('màn Khám phá', () => {
  it('render cột đối thủ, lưới, cột phải (đối thủ + quét + lịch đăng)', async () => {
    const el = await mount(<ExploreScreen route={ROUTE} />)
    await until(() => !!byText(el, '*', 'Một Phim') && !!byText(el, '*', 'Video số 0'))
    expect(byText(el, '*', tt.scanTitle)).toBeTruthy()
    expect(byText(el, '*', tt.postingTitle)).toBeTruthy()
    expect(el.querySelector('.vx-bulk')).toBeNull()
  })

  it('chọn nhiều: checkbox, Top 10/20, Bỏ chọn, lựa chọn sống qua refetch SSE', async () => {
    const el = await mount(<ExploreScreen route={ROUTE} />)
    await until(() => !!byText(el, '*', 'Video số 0'))
    await click(byText(el, 'button', tt.selectMany))

    const boxes = [...el.querySelectorAll<HTMLInputElement>('.vx-check input')]
    expect(boxes.length).toBeGreaterThan(0)
    await click(boxes[0])
    expect(byText(el, '*', tt.selected(1))).toBeTruthy()

    await click(byText(el, 'button', tt.top10))
    expect(byText(el, '*', tt.selected(10))).toBeTruthy()
    await click(byText(el, 'button', tt.top20))
    expect(byText(el, '*', tt.selected(20))).toBeTruthy()

    // SSE `rivals` → trang đầu đổi (1 video mới chen đầu) — lựa chọn giữ nguyên
    const fresh = { videos: [video(999), ...videos.slice(0, 199)], total: videos.length + 1, nextCursor: 'c2' }
    act(() => { setData(keys.rivalVideos('all', 'outlier', 0, 30, ''), fresh) })
    await flush()
    expect(byText(el, '*', 'Video số 999')).toBeTruthy()
    expect(byText(el, '*', tt.selected(20))).toBeTruthy()

    await click(byText(el, 'button', tt.deselect))
    expect(el.querySelector('.vx-bulk')).toBeNull()
  })

  it('điều khiển cần kênh của bạn: khoá bằng aria-disabled, vẫn focus được, lý do qua aria-describedby', async () => {
    const el = await mount(<ExploreScreen route={ROUTE} />)
    await until(() => !!byText(el, '*', 'Video số 0'))
    const cardBtn = [...el.querySelectorAll<HTMLButtonElement>('.vx-card button')].find((b) => b.textContent?.includes(tt.makeVideo))!
    expectLocked(cardBtn)
    expect(await showTip(cardBtn)).toBe(tt.lockedReason)

    await click(byText(el, 'button', tt.selectMany))
    await click(el.querySelector<HTMLInputElement>('.vx-check input'))
    const bulkPrimary = byText(el, 'button', tt.bulkMake(1))!
    expectLocked(bulkPrimary as HTMLButtonElement)
    expect(await showTip(bulkPrimary)).toBe(tt.lockedReason)

    // Chip "Trạng thái": nút khoá, không mở listbox, vẫn là điểm dừng Tab.
    const statusChip = el.querySelector<HTMLButtonElement>('.vx-chip--locked')!
    expectLocked(statusChip)
    expect(statusChip.querySelector('select')).toBeNull()
  })

  it('menu ⋯ của đối thủ: "Liên kết với kênh của tôi" khoá, lý do là dòng phụ, bấm không đóng menu', async () => {
    const el = await mount(<ExploreScreen route={ROUTE} />)
    await until(() => !!byText(el, '*', tt.statBaseline))
    await click(el.querySelector(`[aria-label="${tt.rivalActions('Một Phim')}"]`))
    const item = [...document.body.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')].find((b) => b.textContent?.includes(tt.menuLink))!
    expectLocked(item)
    await click(item)
    expect(document.body.querySelector('[role="menuitem"]')).toBeTruthy()
  })

  it('chip Đăng trong: <select> nằm trong chip nên vòng focus của chip hiện khi chọn bằng phím', async () => {
    const el = await mount(<ExploreScreen route={ROUTE} />)
    await until(() => !!byText(el, '*', 'Video số 0'))
    const chip = el.querySelector<HTMLElement>('.vx-chip--select')!
    const select = chip.querySelector('select')!
    // Vòng focus vẽ bằng .vx-chip:focus-within — chỉ đúng khi select ở trong chip.
    expect(select.parentElement).toBe(chip)
    expect(select.getAttribute('aria-label')).toBe(tt.withinLabel)
    // Chip lọc luôn có mũi tên chỉnh giá trị như artboard (clock/calendar + arrow).
    const icons = [...chip.children].filter((child) => (child as HTMLElement).style.maskImage || (child as HTMLElement).style.webkitMaskImage)
    expect(icons.length).toBe(2)
  })

  it('bấm thẻ video → panel xem trước với Vì sao, Mở trên YouTube, nút chính khoá', async () => {
    const el = await mount(<ExploreScreen route={ROUTE} />)
    await until(() => !!byText(el, '*', 'Video số 0'))
    await click(byText(el, 'button', 'Video số 0'))
    await until(() => !!byText(el, 'button', tt.rivalInfo))
    await click(byText(el, 'button', tt.why))
    expect(byText(el, '*', 'Trung vị kênh')).toBeTruthy()
    // Chọn kênh + nút chính khoá, cùng trỏ tới câu lý do hiện dưới nút (kèm link Tạo kênh).
    const make = byText(el.querySelector('.vx-rail')!, 'button', tt.makeVideo)! as HTMLButtonElement
    expectLocked(make)
    expectLocked(el.querySelector<HTMLElement>('.vx-makeopt')!)
    expect(el.querySelector('.vx-locknote a')?.getAttribute('href')).toBe('#/channels/new')
    const link = el.querySelector('a.vx-linkbtn')!
    expect(link.getAttribute('href')).toBe('https://youtu.be/v0')
    await click(byText(el, 'button', tt.rivalInfo))
    expect(byText(el, '*', tt.statBaseline)).toBeTruthy()
  })

  it('tìm tiêu đề: gõ → chờ debounce rồi hỏi API một lần với q, dòng đếm nói số khớp trong số đã quét', async () => {
    const el = await mount(<ExploreScreen route={ROUTE} />)
    await until(() => !!byText(el, '*', 'Video số 0'))
    expect(el.querySelector('.vx-count')?.textContent).toBe(tt.videoCount('240', '1.284'))
    const box = el.querySelector<HTMLInputElement>('[role="search"] input')!
    for (const partial of ['s', 'số 1', 'Số 12']) await typeIn(box, partial)
    await until(() => searched.includes('Số 12'))
    // Gõ liền tay chỉ hỏi chuỗi cuối.
    expect(searched).toEqual(['Số 12'])
    // "Số 12" khớp Video số 12 và 120–129.
    await until(() => el.querySelector('.vx-count')?.textContent === tt.searchCount('11', 'Số 12', '1.284'))
    expect(byText(el, '.vx-vtitle', 'Video số 0')).toBeUndefined()

    // Nút ✕ cuối ô xoá tìm ngay, không chờ debounce.
    await click(el.querySelector(`[role="search"] button[aria-label="${tt.searchClear}"]`))
    expect(box.value).toBe('')
    await until(() => !!byText(el, '.vx-vtitle', 'Video số 0'))
  })

  it('tìm không ra: khối thông báo nói tìm ở đâu, lọc gì + "Xoá tìm"', async () => {
    const el = await mount(<ExploreScreen route={ROUTE} />)
    await until(() => !!byText(el, '*', 'Video số 0'))
    await typeIn(el.querySelector<HTMLInputElement>('[role="search"] input')!, 'basement')
    await until(() => !!el.querySelector('.vx-nomatch'))
    const block = el.querySelector('.vx-nomatch')!
    expect(block.getAttribute('role')).toBe('status')
    expect(block.textContent).toContain(tt.noMatchTitle('basement'))
    expect(block.textContent).toContain(tt.noMatchWhere('1.284', tt.noMatchAll, [tt.noMatchWithin('30 ngày')]))
    expect(el.querySelector('.vx-count')?.textContent).toBe(tt.searchCountNone('basement'))
    await click(byText(block, 'button', tt.searchClear))
    await until(() => !!byText(el, '.vx-vtitle', 'Video số 0'))
    expect(el.querySelector('.vx-nomatch')).toBeNull()
  })

  it('phím "/" đưa focus vào ô tìm (trừ khi đang gõ ô khác), Esc trong ô xoá chữ', async () => {
    const el = await mount(<ExploreScreen route={ROUTE} />)
    await until(() => !!byText(el, '*', 'Video số 0'))
    const box = el.querySelector<HTMLInputElement>('[role="search"] input')!
    const slash = await press(document.body, '/')
    expect(slash.defaultPrevented).toBe(true)
    expect(document.activeElement).toBe(box)

    const other = el.querySelector<HTMLInputElement>('.vx-add input')!
    other.focus()
    expect((await press(other, '/')).defaultPrevented).toBe(false)
    expect(document.activeElement).toBe(other)

    await typeIn(box, 'floor')
    await press(box, 'Escape')
    expect(box.value).toBe('')
  })

  it('trạng thái trống: tiêu đề riêng, cột đối thủ rỗng, form thêm + POST /api/rivals', async () => {
    listBody = { rivals: [], totalVideos: 0, newOutliers: 0 }
    const el = await mount(<ExploreScreen route={ROUTE} />)
    let input: HTMLInputElement | null = null
    await until(() => { input = el.querySelector('input'); return input !== null })
    // Artboard empty: H1 là tiêu đề của trạng thái, không mượn tên màn.
    expect(el.querySelector('.vx-empty__title')?.textContent).toBe(tt.emptyTitle)
    expect(el.querySelector('.vx-empty__title')?.textContent).not.toBe(tt.title)
    // Cột trái 252 px vẫn còn: nhóm "Đối thủ 0" + câu hướng dẫn, không lặp form.
    const side = el.querySelector('.vx-side')!
    expect(side.textContent).toContain(tt.rivalsGroup)
    expect(side.textContent).toContain(tt.rivalsEmpty)
    expect(side.textContent).toContain('0')
    expect(side.querySelector('form')).toBeNull()
    // Ô thêm chỉ có một chỗ — trong khối giữa.
    expect(el.querySelectorAll('form').length).toBe(1)
    // Ví dụ theo artboard.
    for (const example of tt.emptyExamples) expect(el.textContent).toContain(example)
    await act(async () => {
      const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, 'value')?.set
      setter?.call(input!, '@motphim')
      input!.dispatchEvent(new Event('input', { bubbles: true }))
    })
    await click(byText(el, 'button', tt.addRival))
    await until(() => posted.some((p) => p.url === '/api/rivals' && (p.body as { input?: string }).input === '@motphim'))
  })
})

describe('trạng thái danh sách (gói V34)', () => {
  it('đối thủ đang quét thì hiện băng + ô xương, không báo lọc rỗng', async () => {
    videos = []
    listBody = {
      rivals: [{ ...rival, videoCount: 12, scannedAt: null, scan: { state: 'running', discoveryRunning: false } }],
      totalVideos: 0,
      newOutliers: 0,
    }
    const el = await mount(<ExploreScreen route={ROUTE} />)
    await until(() => !!byText(el, '*', tt.listScanTitle('12', '30')))
    expect(byText(el, '*', tt.listScanTitle('12', '30'))?.textContent).toContain('12 / 30')
    expect(byText(el, '*', tt.listFilterTitle)).toBeFalsy()
    expect(el.querySelector('[data-list-skeleton]')).toBeTruthy()
    // Dòng đếm đọc tiến độ quét thay cho "0 / N video".
    expect(el.textContent).toContain(tt.listCountScanning('12', '30'))
  })

  it('lọc rỗng: câu nêu bộ lọc đang bật, Bỏ lọc đưa Dài và Đăng trong về mặc định', async () => {
    videos = []
    const el = await mount(<ExploreScreen route={ROUTE} />)
    await until(() => !!byText(el, '*', tt.listFilterTitle))
    // Mặc định 30 ngày vẫn là một bộ lọc đang bật.
    expect(byText(el, '*', tt.listFilterText([tt.noMatchWithin(tt.withinDays.d30)], '1.284', tt.noMatchAll))).toBeTruthy()

    const select = el.querySelector(`select[aria-label="${tt.withinLabel}"]`) as HTMLSelectElement
    await act(async () => { select.value = '7'; select.dispatchEvent(new Event('change', { bubbles: true })) })
    await flush()
    const longChip = byText(el, 'button', tt.filterLong)!
    await click(longChip)
    await until(() => el.textContent!.includes(tt.noMatchWithin(tt.withinDays.d7)) && el.textContent!.includes(tt.noMatchLong))
    expect(longChip.getAttribute('aria-pressed')).toBe('true')

    await click(byText(el, 'button', tt.listFilterClear))
    await until(() => {
      const chip = byText(el, 'button', tt.filterLong)!
      const day = el.querySelector(`select[aria-label="${tt.withinLabel}"]`) as HTMLSelectElement
      return chip.getAttribute('aria-pressed') === 'false' && day.value === '30'
    })
    expect((el.querySelector(`select[aria-label="${tt.withinLabel}"]`) as HTMLSelectElement).value).toBe('30')
    expect(byText(el, 'button', tt.filterLong)!.getAttribute('aria-pressed')).toBe('false')
  })

  it('lỗi tải: Thử lại gọi lại danh sách và rời khối lỗi', async () => {
    videos = []
    videosFails = 1
    const el = await mount(<ExploreScreen route={ROUTE} />)
    await until(() => !!byText(el, '*', tt.listErrorTitle))
    expect(byText(el, '*', tt.listFilterTitle)).toBeFalsy()
    const before = vi.mocked(fetch).mock.calls.filter(([input]) => String(input).startsWith('/api/rivals/all/videos')).length
    await click(byText(el, 'button', tt.listRetry))
    await until(() => !byText(el, '*', tt.listErrorTitle))
    const after = vi.mocked(fetch).mock.calls.filter(([input]) => String(input).startsWith('/api/rivals/all/videos')).length
    expect(after).toBeGreaterThan(before)
    // Tải lại xong mà vẫn 0 video (không lọc gì khác) → khối lọc rỗng, không phải lỗi.
    await until(() => !!byText(el, '*', tt.listFilterTitle))
  })
})
