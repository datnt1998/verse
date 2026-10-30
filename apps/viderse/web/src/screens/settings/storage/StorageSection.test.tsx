// @vitest-environment happy-dom
// Cài đặt › Lưu trữ — fixture đúng wire Storage (internal/web/storage.go):
// {projects{path,volume{name,kind,fs,freeBytes},usedBytes,online,offlineSince?,previous?},
//  stayBytes, suggestions[], usage[{id,bytes}], clean{lastRunAt}}.
// Phủ các trạng thái store= của artboard: here · offline · pick/lowspace ·
// clean=preview/run · xoá nhóm có xác nhận cam.
import { act } from 'react'
import { invalidate } from '@noverse/ui/data'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import StorageSection from './StorageSection'
import type { StorageStatus } from '../../../api/storage'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

const roots: Root[] = []
let storage: StorageStatus
let reclaimItems: { id: 'src' | 'mp4'; title: string; bytes: number; count: number; desc: string }[]
let calls: { url: string; method: string; body?: string }[] = []
let checkOk = true
let runningTasks: any[] = []
let checkHandler: ((body: BodyInit | null | undefined) => Promise<Response>) | null = null

const respond = (body: unknown, status = 200) => Promise.resolve(Response.json(body, { status }))

function installFetch() {
  vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
    const method = init?.method ?? 'GET'
    calls.push({ url, method, body: typeof init?.body === 'string' ? init.body : undefined })
    if (url === '/api/storage' && method === 'GET') return respond(storage)
    if (url.startsWith('/api/activity')) return respond({ running: runningTasks, videoJobs: runningTasks.length, queued: 0, unread: [] })
    if (url === '/api/storage/reclaim') return respond({ items: reclaimItems })
    if (url.startsWith('/api/storage/browse')) return respond({ path: '/Users/dat', roots: [{ name: 'Macintosh HD', path: '/Users/dat' }], dirs: [{ name: 'Videos', path: '/Users/dat/Videos' }] })
    if (url === '/api/storage/check') {
      if (checkHandler) return checkHandler(init?.body ?? '')
      return respond({
        ok: checkOk, createSubdir: false, needBytes: 81_000_000_000, etaSec: 480,
        checks: checkOk
          ? [{ kind: 'space', status: 'ok', text: 'Đủ chỗ cho 81,1 GB' }, { kind: 'write', status: 'ok', text: 'Ghi được vào thư mục' }]
          : [{ kind: 'space', status: 'fail', text: 'Không đủ chỗ: cần 81,1 GB, còn 12 GB' }],
      })
    }
    if (url === '/api/storage/cleanup/preview') {
      return respond({ previewId: 'p1', items: [{ kind: 'tmp', label: 'Tệp tạm khi dựng và tải video', count: 12, bytes: 1.8e9 }], freedBytes: 1.8e9, stoppedProcesses: 0 })
    }
    if (url === '/api/storage/cleanup/run') return respond({ removed: 12, freedBytes: 1.8e9, stoppedProcesses: 0 })
    if (url === '/api/storage/open') return respond({ ok: true })
    if (url.startsWith('/api/storage/reclaim/')) return respond(reclaimItems.find((i) => url.endsWith(i.id)))
    if (url === '/api/storage/move') return respond({ taskId: 'm1', done: false })
    return respond({}, 404)
  }))
}

async function render() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(<StorageSection />) })
  for (let i = 0; i < 5; i++) await act(async () => { await new Promise((r) => setTimeout(r, 20)) })
  return container
}

const click = async (el: Element | null | undefined) => {
  expect(el).toBeTruthy()
  await act(async () => { (el as HTMLElement).dispatchEvent(new MouseEvent('click', { bubbles: true })) })
  for (let i = 0; i < 3; i++) await act(async () => { await new Promise((r) => setTimeout(r, 20)) })
}
const byText = (root: ParentNode, sel: string, text: string) =>
  [...root.querySelectorAll(sel)].find((el) => el.textContent?.includes(text))

beforeEach(() => {
  void invalidate()
  calls = []
  checkOk = true
  checkHandler = null
  runningTasks = []
  vi.stubGlobal('matchMedia', vi.fn().mockImplementation(() => ({ matches: false, addEventListener: () => {}, removeEventListener: () => {} })))
  storage = {
    projects: {
      path: '/Users/dat/Viderse Projects',
      volume: { name: 'Macintosh HD', kind: 'internal', fs: 'apfs', freeBytes: 511_000_000_000 },
      usedBytes: 86_400_000_000,
      online: true,
    },
    stayBytes: 5_300_000_000,
    suggestions: [],
    usage: [{ id: 'sources', bytes: 38_200_000_000 }, { id: 'exports', bytes: 27_400_000_000 }],
    move: { active: false, taskId: '', failed: null },
    clean: { lastRunAt: '2026-10-30T22:14:00Z' },
  }
  reclaimItems = [
    { id: 'src', title: 'Video gốc của video đã xong', bytes: 31_600_000_000, count: 14, desc: 'Xoá video tải từ đối thủ của 14 video đã xong.' },
    { id: 'mp4', title: 'MP4 của video đã đăng', bytes: 14_800_000_000, count: 9, desc: '9 video đã đăng lên YouTube.' },
  ]
  installFetch()
})

afterEach(async () => {
  await invalidate()
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  vi.unstubAllGlobals()
})

describe('Lưu trữ · store=here', () => {
  it('thẻ dự án: tên ổ + loại, đường dẫn cắt giữa có title, Mở trong Finder, Đổi thư mục', async () => {
    const el = await render()
    const loc = el.querySelector('.vv-loc')!
    expect(loc.textContent).toContain('Macintosh HD')
    expect(loc.textContent).toContain('Ổ trong máy')
    const path = loc.querySelector('.vv-loc__path')!
    expect(path.getAttribute('title')).toBe('/Users/dat/Viderse Projects')
    expect(loc.textContent).toContain('Mở trong Finder')
    expect(loc.textContent).toContain('Đổi thư mục…')
    await click(byText(el, 'button', 'Mở trong Finder'))
    expect(calls.some((c) => c.url === '/api/storage/open')).toBe(true)
  })

  it('biểu đồ dung lượng có nhóm và chỗ trống', async () => {
    const el = await render()
    expect(el.textContent).toContain('Viderse đang dùng 91,7 GB')
    expect(el.textContent).toContain('Video gốc tải về')
    expect(el.textContent).toContain('Mô hình và công cụ')
    expect(el.textContent).toContain('Ổ còn trống')
  })

  it('ổ ngoài nêu cổng nối khi máy chủ biết', async () => {
    storage = {
      ...storage,
      projects: { ...storage.projects, volume: { ...storage.projects.volume, name: 'Samsung T7', kind: 'external', fs: 'APFS', bus: 'USB' } },
    }
    const el = await render()
    expect(el.textContent).toContain('Ổ ngoài USB · APFS')
  })
})

describe('Lưu trữ · store=offline', () => {
  it('banner lỗi nêu tên ổ, giờ rút, và hai lối thoát', async () => {
    storage = {
      ...storage,
      projects: { ...storage.projects, online: false, offlineSince: '2026-10-30T21:40:00Z', volume: { ...storage.projects.volume, name: 'Samsung T7', kind: 'external' } },
    }
    const el = await render()
    expect(el.textContent).toContain('Không thấy ổ Samsung T7')
    expect(el.textContent).toContain('Thử lại')
    expect(el.textContent).toContain('Lưu video mới vào')
    // Ổ rút: dòng vị trí vẫn hiện (để Mở trong Finder), riêng “Đổi thư mục” mờ.
    expect(el.textContent).toContain('Mở trong Finder')
    expect(byText(el, 'button', 'Đổi thư mục…')).toBeUndefined()
  })
})

describe('Lưu trữ · store=pick', () => {
  it('chọn thư mục → check đạt → CTA chuyển bật, gọi /api/storage/move', async () => {
    const el = await render()
    await click(byText(el, 'button', 'Đổi thư mục…'))
    // store=pick là khối chọn tại chỗ trong thẻ (artboard), không phải dialog.
    expect(el.textContent).toContain('Thư mục mới')
    expect(el.textContent).toContain('Dữ liệu đang có')
    expect(el.textContent).toContain('Đủ chỗ cho 81,1 GB')
    const submit = byText(el, 'button', 'Chuyển sang thư mục mới') as HTMLButtonElement
    expect(submit.disabled).toBe(false)
    await click(submit)
    const move = calls.find((c) => c.url === '/api/storage/move')
    expect(move?.body).toContain('"mode":"move"')
  })

  it('store=lowspace: check hỏng thì CTA khoá kèm lý do', async () => {
    checkOk = false
    const el = await render()
    await click(byText(el, 'button', 'Đổi thư mục…'))
    expect(el.textContent).toContain('Không đủ chỗ: cần 81,1 GB, còn 12 GB')
    const submit = byText(el, 'button', 'Chuyển sang thư mục mới') as HTMLButtonElement
    expect(submit.disabled).toBe(true)
    // Chân khối chọn nói cách thoát (câu khôi phục của artboard), không lặp lỗi.
    expect(el.querySelector('.vv-sto__reason')?.textContent).toContain('Chọn thư mục khác')
  })

  it('đổi ổ giữa chờ check: kết quả trễ của ổ cũ không mở lại CTA', async () => {
    storage = {
      ...storage,
      suggestions: [
        { path: '/Volumes/T7/Viderse Projects', volume: { name: 'Samsung T7', kind: 'external', fs: 'apfs', freeBytes: 1_540_000_000_000 } },
        { path: '/Volumes/Lexar/Viderse Projects', volume: { name: 'Lexar', kind: 'external', fs: 'exfat', freeBytes: 12_000_000_000 } },
      ],
    }
    // T7 trả chậm và đạt; Lexar trả nhanh nhưng thiếu chỗ — kết quả trễ của T7
    // tới sau không được lật lại checklist thành công hay mở CTA.
    checkHandler = async (body) => {
      const path = JSON.parse(String(body))?.path as string
      if (path.includes('Lexar')) {
        return respond({ ok: false, createSubdir: false, needBytes: 81_000_000_000, etaSec: 480,
          checks: [{ kind: 'space', status: 'fail', text: 'Không đủ chỗ: cần 81,1 GB, còn 12 GB' }] })
      }
      await new Promise<void>((r) => setTimeout(r, 400))
      return respond({ ok: true, createSubdir: false, needBytes: 81_000_000_000, etaSec: 480,
        checks: [{ kind: 'space', status: 'ok', text: 'Đủ chỗ cho 81,1 GB' }] })
    }
    const el = await render()
    await click(byText(el, 'button', 'Đổi thư mục…'))
    await click(byText(el, 'button', 'Lexar'))
    expect(el.textContent).toContain('Không đủ chỗ: cần 81,1 GB, còn 12 GB')
    const submit = byText(el, 'button', 'Chuyển sang thư mục mới') as HTMLButtonElement
    expect(submit.disabled).toBe(true)
    // Đợi phản hồi trễ của T7 tới — nó phải bị bỏ qua.
    await act(async () => { await new Promise<void>((r) => setTimeout(r, 450)) })
    expect(el.textContent).not.toContain('Đủ chỗ cho 81,1 GB')
    expect((byText(el, 'button', 'Chuyển sang thư mục mới') as HTMLButtonElement).disabled).toBe(true)
  })
})

describe('Lưu trữ · dọn dẹp và xoá nhóm', () => {
  it('Xem trước → Dọn, hai bước previewId', async () => {
    const el = await render()
    await click(byText(el, 'button', 'Xem trước'))
    expect(el.textContent).toContain('Tệp tạm khi dựng và tải video')
    await click(byText(el, 'button', 'Dọn 1,8 GB'))
    const run = calls.find((c) => c.url === '/api/storage/cleanup/run')
    expect(run?.body).toContain('"previewId":"p1"')
    expect(el.textContent).toContain('Đã dọn')
  })

  it('xoá nhóm phải qua xác nhận cam: Huỷ không xoá, xác nhận mới gọi', async () => {
    const el = await render()
    await click(byText(el, 'button', 'Xoá video gốc'))
    expect(el.querySelector('[role="alertdialog"]')).toBeTruthy()
    expect(el.textContent).toContain('Không hoàn tác được')
    await click(byText(el, 'button', 'Huỷ'))
    expect(calls.some((c) => c.url.startsWith('/api/storage/reclaim/'))).toBe(false)

    await click(byText(el, 'button', 'Xoá video gốc'))
    await click(el.querySelector('[data-confirm]'))
    expect(calls.some((c) => c.url === '/api/storage/reclaim/src' && c.method === 'POST')).toBe(true)
    expect(el.textContent).toContain('Đã xoá · lấy lại')
  })

  it('nhóm count=0 vẫn hiện thẻ nhưng khoá nút kèm lý do', async () => {
    reclaimItems = reclaimItems.map((i) => ({ ...i, count: 0, bytes: 0 }))
    const el = await render()
    expect(el.textContent).toContain('Video gốc của video đã xong')
    expect(el.textContent).toContain('Chưa có video nào thuộc nhóm này.')
    const del = byText(el, 'button', 'Xoá video gốc') as HTMLButtonElement
    expect(del.disabled).toBe(true)
  })

  it('store=donenew: banner đầy đủ path + GB và KHÔNG có nút xoá bản cũ', async () => {
    storage = {
      ...storage,
      projects: {
        ...storage.projects,
        previous: { path: '/Users/dat/Old Projects', bytes: 17_200_000_000, mode: 'new_only' },
        lastSwitch: { mode: 'new_only', at: '2026-10-30T09:00:00Z' },
      },
    }
    const el = await render()
    expect(el.textContent).toContain('Video mới lưu ở Macintosh HD')
    expect(el.textContent).toContain('17,2 GB')
    // Câu banner nêu *ổ* của dữ liệu cũ (artboard: “vẫn ở Macintosh HD”), không
    // phải đường dẫn — đường dẫn không khớp gợi ý nào thì rơi về ổ trong máy.
    expect(el.textContent).toContain('dự án cũ vẫn ở Macintosh HD')
    expect(byText(el, 'button', 'Xoá bản cũ ngay')).toBeUndefined()
    expect(byText(el, 'button', 'Chuyển về')).toBeUndefined()
  })

  it('store=done: có nút xoá bản cũ ngay và chuyển về', async () => {
    storage = {
      ...storage,
      projects: {
        ...storage.projects,
        previous: { path: '/Volumes/T7/Projects', bytes: 81_100_000_000, mode: 'move', deleteAfter: '2026-11-06T09:00:00Z' },
        lastSwitch: { mode: 'move', at: '2026-10-30T09:00:00Z' },
      },
    }
    const el = await render()
    expect(el.textContent).toContain('Đã chuyển 81,1 GB')
    expect(byText(el, 'button', 'Xoá bản cũ ngay')).toBeTruthy()
    expect(byText(el, 'button', 'Chuyển về')).toBeTruthy()
  })

  it('offline ở thư mục mặc định trong máy: “Không thấy thư mục dự án”', async () => {
    storage = { ...storage, projects: { ...storage.projects, online: false, offlineSince: '2026-10-30T21:40:00Z' } }
    const el = await render()
    expect(el.textContent).toContain('Không thấy thư mục dự án')
  })

  it('check trả volume_gone: banner cắm lại thay cho bảng kiểm tra', async () => {
    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url
      const method = init?.method ?? 'GET'
      calls.push({ url, method, body: typeof init?.body === 'string' ? init.body : undefined })
      if (url === '/api/storage' && method === 'GET') return respond(storage)
      if (url.startsWith('/api/activity')) return respond({ running: runningTasks, videoJobs: runningTasks.length, queued: 0, unread: [] })
      if (url === '/api/storage/reclaim') return respond({ items: reclaimItems })
      if (url.startsWith('/api/storage/browse')) return respond({ path: '/Volumes/T7', roots: [], dirs: [] })
      if (url === '/api/storage/check') return respond({ error: 'không thấy ổ', code: 'volume_gone' }, 404)
      return respond({}, 404)
    }))
    const el = await render()
    await click(byText(el, 'button', 'Đổi thư mục…'))
    expect(el.textContent).toContain('Ổ chưa cắm hoặc đã rút')
    expect(el.textContent).not.toContain('Đủ chỗ cho')
  })

  it('banner đếm *việc nền*, không gọi các task đang chạy là video (hỏi-đáp #23)', async () => {
    runningTasks = [
      { id: 't1', kind: 'video', title: 'Video 1', objectRef: { type: 'video', id: 't1', label: 'Video 1' }, phase: 'Đang làm', progress: null, etaSec: null, state: 'running', action: { label: 'Tạm dừng' } },
      { id: 't2', kind: 'upload', title: 'Tải lên', objectRef: { type: 'upload', id: 't2', label: 'Tải lên' }, phase: 'Đang tải', progress: null, etaSec: null, state: 'running', action: { label: 'Tạm dừng' } },
    ]
    storage = {
      ...storage,
      suggestions: [{ path: '/Volumes/T7/Viderse Projects', volume: { name: 'T7', kind: 'external', fs: 'APFS', freeBytes: 1_540_000_000_000 } }],
    }
    const el = await render()
    await click(byText(el, 'button', 'Đổi thư mục…'))
    // Số đếm là task của /api/activity, nên câu phải nói “việc nền”.
    expect(el.textContent).toContain('2 việc nền đang chạy')
    expect(el.textContent).not.toMatch(/\d+ video đang chạy/)
  })

  it('thẻ chuyển thư mục cũng đếm *việc nền*', async () => {
    runningTasks = [
      { id: 'm1', kind: 'storage_move', title: 'Đang chuyển sang T7', objectRef: { type: 'storage', id: 'm1', label: 'T7' }, phase: 'Đang chuyển', progress: { done: 1e9, total: 2e9, groups: [] }, etaSec: 120, state: 'running', action: { label: 'Huỷ chuyển' } },
      { id: 't2', kind: 'upload', title: 'Tải lên', objectRef: { type: 'upload', id: 't2', label: 'Tải lên' }, phase: 'Đang tải', progress: null, etaSec: null, state: 'running', action: { label: 'Tạm dừng' } },
    ]
    storage = { ...storage, move: { active: true, taskId: 'm1', failed: null } }
    const el = await render()
    expect(el.textContent).toContain('1 việc nền đang tạm dừng')
    expect(el.textContent).not.toContain('video đang tạm dừng')
  })
})
