// @vitest-environment happy-dom
// Màn Kênh — hành vi khoá/lưu/lỗi:
// 1. Tạo kênh cần ≥ 1 đối thủ: nút aria-disabled + lý do, chọn xong mở khoá;
// 2. Hồ sơ kênh tự lưu khi rời ô; 400 theo trường giữ nguyên chữ đang gõ;
// 3. Bỏ đối thủ cuối → gọi DELETE nguồn;
// 4. Nhận diện đang vẽ thì nút vẽ lại khoá kèm lý do; lỗi hiện giờ tự thử lại;
// 5. Khám phá: có kênh thì chọn được kênh đích trước khi làm hàng loạt.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { invalidate } from '@noverse/ui/data'
import { keys } from '../../data/keys'
import { t } from '../../i18n/all'
import type { Activity } from '../../api/activity'
import type { Rival, RivalList } from '../../api/rivals'
import type { Channel } from '../../api/channels'
import { CreateStep } from './CreateStep'
import { DoneStep } from './DoneStep'
import { SettingsTab } from './SettingsTab'
import { SourcesTab } from './SourcesTab'
import { IdentityBlock } from './IdentityBlock'
import { PreviewPanel } from '../explore/PreviewPanel'
import { RivalCard } from '../explore/RivalCard'
import { FilterBar } from '../explore/FilterBar'
import { BulkBar } from '../explore/BulkBar'

const tt = t.channels
const et = t.screens.explore

const rival: Rival = {
  id: 'r1', handle: '@motphim', title: 'Một Phim', avatarUrl: null,
  url: 'https://youtube.com/@motphim', subscriberCount: 1_240_000,
  videoCount: 1284, medianViews: 286_000, outliers: 42, newOutliers: 4,
  addedAt: '2026-10-20T00:00:00Z', scannedAt: '2026-10-29T08:00:00Z',
  scan: { state: 'idle', discoveryRunning: false },
}

const channel = (patch: Partial<Channel> = {}): Channel => ({
  id: 'c1', name: 'Chuyện Lạ', youtubeUrl: null, language: 'vi',
  styleNote: null, descriptionFooter: null, alwaysTags: [],
  autoPick: true, pacePerDay: 5, templateCursor: 0,
  autonomy: { preset: 'balanced', pick: 'all', approve: { mode: 'ask', min: 85 }, onError: 'retry_once', belowQuality: 'ask' }, priority: 'steady',
  branding: { brief: '', summary: '', look: '', avatar: '', banner: '' },
  avatarUrl: null, bannerUrl: null,
  sources: [{ rivalId: 'r1', title: 'Một Phim', handle: '@motphim', linkedAt: '2026-10-20T00:00:00Z', reference: true }],
  templates: [], identity: { state: 'ready' },
  createdAt: '2026-10-20T00:00:00Z', updatedAt: '2026-10-20T00:00:00Z',
  ...patch,
})

const activity = (percentDone: number, total: number, id = 'c1'): Activity => ({
  running: [{
    id: 'job1', kind: 'identity', agent: 'artist', title: 'Vẽ nhận diện',
    objectRef: { type: 'channel', id, label: 'Chuyện Lạ' }, phase: 'vẽ avatar',
    progress: { done: percentDone, total }, etaSec: 60, state: 'running',
    action: { label: 'Huỷ', op: 'cancel' },
  }],
  videoJobs: 0, queued: 0, unread: [],
})

const respond = (body: unknown, status = 200) =>
  Promise.resolve(new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: body === undefined ? {} : { 'content-type': 'application/json' },
  }))

let posts: { url: string; method: string; body: unknown }[] = []

/** fetch giả: mặc định các endpoint đọc trả dữ liệu, ghi ghi lại body để soát. */
function stubFetch(handlers: Record<string, (init?: RequestInit, url?: string) => Promise<Response>>) {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    const method = init?.method ?? 'GET'
    if (method !== 'GET' || init?.body) posts.push({ url, method, body: init?.body ? JSON.parse(String(init.body)) : undefined })
    for (const [prefix, handler] of Object.entries(handlers)) {
      if (url === prefix || url.startsWith(prefix)) return handler(init, url)
    }
    return respond({ error: 'not found' }, 404)
  }))
}

const roots: Root[] = []
const flush = async (ms = 30) => {
  await act(async () => {
    await new Promise<void>((resolve) => { setTimeout(resolve, ms) })
  })
}

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

const until = async (fn: () => boolean, tries = 40) => {
  for (let i = 0; i < tries; i++) {
    if (fn()) return
    await flush()
  }
  expect(fn()).toBe(true)
}

const typeIn = async (input: HTMLInputElement, value: string) => {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set
  await act(async () => {
    setter?.call(input, value)
    input.dispatchEvent(new Event('input', { bubbles: true }))
  })
  await flush(5)
}

const leave = async (el: HTMLElement) => {
  await act(async () => {
    el.focus()
    el.dispatchEvent(new FocusEvent('blur', { bubbles: false }))
    el.dispatchEvent(new FocusEvent('focusout', { bubbles: true }))
  })
  await flush()
}

beforeEach(async () => {
  posts = []
  await invalidate(keys.channels())
  await invalidate(keys.rivals())
  await invalidate(keys.channel('c1'))
  await invalidate(keys.channel('c2'))
  await invalidate(keys.channel('c3'))
  await invalidate(keys.templates())
  await invalidate(keys.activity())
})

afterEach(async () => {
  for (const root of roots.splice(0)) await act(async () => { root.unmount() })
  document.body.innerHTML = ''
  vi.unstubAllGlobals()
})

describe('Tạo kênh', () => {
  it('cần ít nhất 1 đối thủ: nút khoá + lý do, chọn rồi mở khoá và gửi đúng', async () => {
    stubFetch({
      '/api/rivals': () => respond({ rivals: [rival], totalVideos: 1284, newOutliers: 4 } satisfies RivalList),
      '/api/templates': () => respond({ items: [] }),
      '/api/owner-channels': () => respond({ channel: channel(), identityJobId: 'job1' }, 201),
    })
    let created = ''
    const el = await mount(<CreateStep firstRun={false} onCancel={() => {}} onCreated={(id) => { created = id }} />)
    await until(() => el.querySelector('[role="checkbox"]') !== null)

    const submit = byText(el, 'button', tt.create.submit) as HTMLButtonElement
    expect(submit.getAttribute('aria-disabled')).toBe('true')
    expect(el.textContent).toContain(tt.create.noteBlocked)

    await click(el.querySelector('[role="checkbox"]'))
    expect(submit.getAttribute('aria-disabled')).toBeNull()
    expect(el.textContent).toContain(tt.create.noteReady)

    await click(submit)
    await until(() => created !== '')
    const post = posts.find((item) => item.url === '/api/owner-channels')
    expect(post?.body).toMatchObject({ competitorIds: ['r1'], pacePerDay: 5 })
    expect(created).toBe('c1')
  })
})

describe('Hồ sơ kênh', () => {
  it('tự lưu khi rời ô: 400 theo trường giữ nguyên chữ đang gõ và hiện lỗi dưới ô', async () => {
    stubFetch({
      '/api/owner-channels/c2': (_init, url) => (
        url === '/api/owner-channels/c2'
          ? respond({ error: 'tên kênh quá dài', code: 'invalid', fields: [{ field: 'name', message: 'Tên kênh tối đa 200 ký tự.' }] }, 400)
          : respond({ error: 'not found' }, 404)
      ),
    })
    const el = await mount(<SettingsTab channel={channel({ id: 'c2', name: 'Tên cũ' })} />)
    const input = el.querySelector('#vch-name') as HTMLInputElement
    expect(input).toBeTruthy()
    await typeIn(input, 'Tên mới đang gõ')
    await leave(input)

    await until(() => Boolean(byText(el, '[role="alert"]', 'Tên kênh tối đa 200 ký tự.')))
    expect((el.querySelector('#vch-name') as HTMLInputElement).value).toBe('Tên mới đang gõ')
    expect(posts.find((item) => item.method === 'PATCH')?.body).toMatchObject({ name: 'Tên mới đang gõ' })
  })
})

describe('Nguồn & tự động', () => {
  it('bỏ đối thủ cuối: hỏi xác nhận rồi gọi DELETE nguồn', async () => {
    stubFetch({
      '/api/rivals': () => respond({ rivals: [rival], totalVideos: 1284, newOutliers: 4 } satisfies RivalList),
      '/api/owner-channels/c3/sources/r1': () => respond({ autoPickPaused: true }),
    })
    const el = await mount(<SourcesTab channel={channel({ id: 'c3' })} channels={[channel({ id: 'c3' })]} />)
    await until(() => el.textContent?.includes('Một Phim') === true)

    await click(byText(el, 'button', tt.sources.unlink))
    await until(() => document.querySelector('[role="alertdialog"]') !== null)
    const dialog = document.querySelector('[role="alertdialog"]')
    expect(dialog).toBeTruthy()
    expect(dialog?.textContent).toContain(tt.sources.unlinkTitle('Một Phim'))

    await click([...(dialog?.querySelectorAll('button') ?? [])].find((b) => b.textContent?.trim() === tt.sources.unlinkOk))
    await until(() => posts.some((item) => item.method === 'DELETE' && item.url.includes('/sources/r1')))
    expect(posts.some((item) => item.method === 'DELETE' && item.url.includes('/sources/r1'))).toBe(true)
    // chờ hộp xác nhận của kit tự đóng + gỡ root trước khi dọn DOM của test
    await until(() => document.querySelector('[role="alertdialog"]') === null)
  })
})

describe('Nguồn: dòng phụ và popover liên kết', () => {
  const scan = {
    channelId: 'r1', historyCap: 2000, indexedVideos: 1284, rescanHours: 6,
    history: { state: 'running', jobId: 'j1', scanned: 1284, added: 1200, pages: 64, reachedEnd: false, startedAt: null, updatedAt: null, nextAttemptAt: null, errorCode: null, error: null, recovery: null },
    discovery: { checkedAt: null, caughtUp: false, running: false, nextAt: null },
  }
  const posting = {
    channelId: 'r1', zone: 'vn', sample: 30, total: 6, pending: 0, pausedUntil: null,
    perWeek: 3, weeksTotal: 6, offSlot: 0,
    grid: Array.from({ length: 7 }, () => Array.from({ length: 12 }, () => 0)),
    slots: [{ weekdays: [1], start: '10:00', end: '11:00', count: 9, weeksHit: 6, weeksTotal: 6, sinceWeeks: 0, rhythm: 'weekly', isTop: true }],
  }

  it('dòng phụ có nhịp video/tuần và tiến độ quét + chip "Đang quét lịch sử"', async () => {
    stubFetch({
      '/api/rivals/r1/scan': () => respond(scan),
      '/api/rivals/r1/posting-slots': () => respond(posting),
      '/api/rivals': () => respond({ rivals: [rival], totalVideos: 1284, newOutliers: 4 } satisfies RivalList),
    })
    const el = await mount(<SourcesTab channel={channel({ id: 'c4' })} channels={[channel({ id: 'c4' })]} />)
    await until(() => el.textContent?.includes(tt.sources.scanChip) === true)
    expect(el.textContent).toContain('3 video/tuần')
    expect(el.textContent).toContain('đang quét lịch sử 1.284/2.000')
  })

  it('hộp chọn là popover trong thẻ: aria-haspopup/expanded, Esc đóng và trả tiêu điểm về nút mở', async () => {
    stubFetch({
      '/api/rivals/r1/scan': () => respond(scan),
      '/api/rivals/r1/posting-slots': () => respond(posting),
      '/api/rivals': () => respond({ rivals: [rival], totalVideos: 1284, newOutliers: 4 } satisfies RivalList),
    })
    const el = await mount(<SourcesTab channel={channel({ id: 'c5' })} channels={[channel({ id: 'c5' })]} />)
    const trigger = el.querySelector<HTMLButtonElement>('[aria-haspopup="dialog"]')!
    expect(trigger.getAttribute('aria-haspopup')).toBe('dialog')
    expect(trigger.getAttribute('aria-expanded')).toBe('false')

    await click(trigger)
    const popover = el.querySelector('.vch-linkslot [role="dialog"]')
    expect(popover).toBeTruthy()
    expect(trigger.getAttribute('aria-expanded')).toBe('true')
    expect(popover?.querySelector('[role="checkbox"]')).toBeTruthy()

    await act(async () => { window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' })) })
    await flush()
    expect(el.querySelector('.vch-linkpop')).toBeNull()
    expect(document.activeElement).toBe(trigger)
  })
})

describe('Nhận diện', () => {
  it('đang vẽ: nút vẽ lại khoá kèm lý do và có tiến độ', async () => {
    stubFetch({ '/api/activity': () => respond(activity(2, 4)) })
    const el = await mount(
      <IdentityBlock channel={channel({ identity: { state: 'generating', startedAt: '2026-10-31T06:00:00Z' } })} />,
    )
    await until(() => el.textContent?.includes(tt.identity.generating) === true)
    const regen = byText(el, 'button', tt.identity.regenAvatar) as HTMLButtonElement
    expect(regen.getAttribute('aria-disabled')).toBe('true')
    const ids = (regen.getAttribute('aria-describedby') ?? '').split(/\s+/).filter(Boolean)
    expect(ids.map((id) => document.getElementById(id)?.textContent).join(' ')).toContain(tt.identity.generatingLock)
    await until(() => el.textContent?.includes('50%') === true)
  })

  it('lỗi: role=alert kèm giờ tự thử lại', async () => {
    const retryAt = new Date(2026, 9, 31, 7, 0).toISOString()
    const el = await mount(
      <IdentityBlock channel={channel({
        identity: { state: 'failed', error: { code: 'quota', message: 'Hết hạn mức ảnh.' }, retryAt, part: 'all' },
      })} />,
    )
    const alert = el.querySelector('[role="alert"]')
    expect(alert?.textContent).toContain('Hết hạn mức ảnh.')
    expect(alert?.textContent).toContain(tt.identity.retryAt('07:00'))
    expect(byText(el, 'button', tt.identity.retry)).toBeTruthy()
  })

  it.each([
    ['images', 'Hết hạn mức ảnh.', 'artist'],
    ['text', 'Tạo nhận diện hỏng.', 'director'],
    [undefined, 'Lần tạo nhận diện trước dừng giữa chừng.', 'director'],
  ] as const)('màn Xong: hỏng ở bước %s → báo lỗi đúng trợ lý', async (step, message, blamed) => {
    const failed = channel({ identity: { state: 'failed', error: { code: 'x', message, step } } })
    stubFetch({
      '/api/owner-channels/c1': () => respond(failed),
      '/api/owner-channels': () => respond({ channels: [failed] }),
      '/api/rivals/': () => respond({ videos: [], total: 0 }),
    })
    const el = await mount(<DoneStep channelId="c1" onLeave={() => {}} />)
    await until(() => el.querySelectorAll('.vch-task').length === 3)
    const [director, artist] = [...el.querySelectorAll('.vch-task')]
    const failedRow = blamed === 'artist' ? artist : director
    const otherRow = blamed === 'artist' ? director : artist
    expect(failedRow.textContent).toContain(message)
    expect(failedRow.querySelector('.vch-task__state--fail')).toBeTruthy()
    expect(otherRow.textContent).not.toContain(message)
    expect(otherRow.textContent).toContain(blamed === 'artist' ? tt.done.ok : tt.done.waitDirector)
  })
})

describe('Khám phá mở khoá theo kênh', () => {
  it('có kênh: chọn được kênh đích, chưa có kế hoạch hợp lệ thì chưa làm video', async () => {
    const el = await mount(
      <PreviewPanel
        video={{
          id: 'v1', channelId: 'r1', title: 'Video mẫu', durationSec: 600, viewCount: 1000,
          publishedAt: '2026-10-28T04:00:00Z', exactTime: true, score: 4.2,
          hasCaptions: null, thumbnailUrl: '', url: 'https://youtu.be/v1',
        }}
        rival={rival}
        channels={[{ id: 'c1', name: 'Chuyện Lạ' }]}
        onClose={() => {}}
      />,
    )
    const radio = el.querySelector('[role="radio"][aria-checked="true"]')
    expect(radio).not.toBeNull()
    const make = byText(el, 'button', et.makeVideo) as HTMLButtonElement
    expect(make.getAttribute('aria-disabled')).toBe('true')
  })

  it('chưa có kênh: giữ lý do cũ và lối tạo kênh', async () => {
    const el = await mount(
      <PreviewPanel
        video={{
          id: 'v1', channelId: 'r1', title: 'Video mẫu', durationSec: 600, viewCount: 1000,
          publishedAt: '2026-10-28T04:00:00Z', exactTime: true, score: 4.2,
          hasCaptions: null, thumbnailUrl: '', url: 'https://youtu.be/v1',
        }}
        rival={rival}
        channels={[]}
        onClose={() => {}}
      />,
    )
    expect(el.textContent).toContain(et.lockedReason)
    expect(el.querySelector('a[href="#/channels/new"]')).toBeTruthy()
  })

  it('mở "Liên kết với kênh…" khi có kênh và liên kết đối thủ vào kênh đã chọn', async () => {
    stubFetch({
      '/api/owner-channels/c1/sources': () => respond(channel()),
    })
    const el = await mount(<RivalCard rival={rival} color="var(--vv-rav-1)" rescanHours={6} perWeek={2.3} channels={[{ id: 'c1', name: 'Chuyện Lạ' }]} onRemoved={() => {}} />)
    await click(el.querySelector('[aria-label="Thao tác với Một Phim"]'))
    const menu = document.querySelector('[role="menu"]')
    const item = [...(menu?.querySelectorAll('[role="menuitem"]') ?? [])].find((b) => b.textContent?.includes(et.menuLink))
    expect(item?.getAttribute('aria-disabled')).toBeNull()
    await click(item)
    const dialog = document.querySelector('[role="dialog"]')
    expect(dialog?.textContent).toContain(et.linkDialogTitle('Một Phim'))
    await click([...(dialog?.querySelectorAll('button') ?? [])].find((b) => b.textContent?.trim() === et.linkPick))
    await until(() => posts.some((item) => item.url === '/api/owner-channels/c1/sources'))
    expect(posts.find((item) => item.url === '/api/owner-channels/c1/sources')?.body).toEqual({ rivalId: 'r1' })
  })

  it('keeps unavailable status filtering locked but enqueues bulk videos for the chosen channel', async () => {
    const filter = await mount(
      <FilterBar sort="newest" long={false} within={30} onSort={() => {}} onLong={() => {}} onWithin={() => {}}
        total={10} scanned={10} q="" hasChannel />,
    )
    const chip = filter.querySelector('.vx-chip--locked') as HTMLElement
    expect(chip.getAttribute('aria-disabled')).toBe('true')
    const chipReason = (chip.getAttribute('aria-describedby') ?? '').split(/\s+/).filter(Boolean)
      .map((id) => document.getElementById(id)?.textContent).join(' ')
    expect(chipReason).toContain(et.queueReason)

    const make = vi.fn()
    const bulk = await mount(
      <BulkBar count={3} selected={[]} onTop={() => {}} onClear={() => {}} onExit={() => {}}
        channels={[{ id: 'c1', name: 'Kênh một' }, { id: 'c2', name: 'Kênh hai' }]} onMake={make} />,
    )
    await click(bulk.querySelector('[aria-haspopup="menu"]'))
    await click(bulk.querySelectorAll('[role="menuitem"]')[1])
    expect(make).toHaveBeenCalledExactlyOnceWith('c2')
  })
})

// r38 — Đạo diễn chưa đặt được tên và mô tả: kênh VẪN được tạo với tên tạm
// "Kênh mới", bước Xong không bị chặn; thẻ đổi thành "Một việc cần bạn" và
// Thử lại chạy lại riêng bước đặt tên của Đạo diễn (part=name).
describe('màn Xong: Đạo diễn lỗi (r38)', () => {
  const failed = channel({
    name: '',
    identity: { state: 'failed', error: { code: 'x', message: 'ChatGPT không phản hồi', step: 'text' } },
  })

  it('tên tạm "Kênh mới" + "Một việc cần bạn"; Thử lại gửi part=name rồi về trạng thái chạy nền', async () => {
    stubFetch({
      '/api/owner-channels/c1/branding': () => respond({ channel: channel({ identity: { state: 'generating' } }), identityJobId: 'job9' }),
      '/api/owner-channels/c1': () => respond(failed),
      '/api/owner-channels': () => respond({ channels: [failed] }),
      '/api/rivals/': () => respond({ videos: [], total: 0 }),
      '/api/activity': () => respond({ running: [], videoJobs: 0, queued: 0, unread: [] }),
    })
    const el = await mount(<DoneStep channelId="c1" onLeave={() => {}} />)
    await until(() => el.querySelectorAll('.vch-task').length === 3)
    expect(el.querySelector('h1')!.textContent).toBe(tt.done.titleTemp)
    expect(el.textContent).toContain(tt.done.noName)
    expect(el.textContent).toContain(tt.done.tempNote)
    expect(el.textContent).toContain(tt.done.needsYou)
    expect(el.textContent).toContain(tt.done.needsYouSub)
    expect(el.textContent).toContain(tt.done.footnoteTemp)
    expect(byText(el, 'button', tt.done.nameInProfile)).toBeTruthy()
    const [director, artist] = [...el.querySelectorAll('.vch-task')]
    expect(director.textContent).toContain('ChatGPT không phản hồi')
    expect(artist.textContent).toContain(tt.done.wait)
    await click(byText(director, 'button', t.common.retry))
    await until(() => posts.some((post) => post.url === '/api/owner-channels/c1/branding'))
    expect(posts.find((post) => post.url === '/api/owner-channels/c1/branding')!.body).toEqual({ part: 'name' })
    await until(() => el.querySelector('h1')!.textContent === tt.done.title)
    expect(el.textContent).toContain(tt.done.background)
  })
})
