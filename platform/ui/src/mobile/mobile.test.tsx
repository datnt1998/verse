// @vitest-environment happy-dom
// Hành vi bộ mobile kit: khoá lời hứa của tấm thao tác, hộp xác nhận alertdialog,
// thanh chọn ẩn thanh tab, banner mất mạng bền 3 giây, bước số kẹp biên, ô nhập 16px.
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ActionSheet, ConfirmSheet } from './sheets'
import { SelectionBar, OfflineBanner } from './bars'
import { InitialsAvatar, ListRow, MeterBar, StatusPill, Stepper, VideoRow, initialsOf } from './rows'
import { MobilePage } from './page'
import { MobileRuntimeProvider, useMobileRuntime } from './runtime'
import { Field } from '../Field'
import { Chip } from '../Chip'
import { Button } from '../Button'
import { setKitLabels } from '../labels'
import { KIT_LABELS_VI } from '../labels.vi'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
setKitLabels(KIT_LABELS_VI)

/** Cổng lời hứa (lib TS của gói chưa có Promise.withResolvers). */
function deferred() {
  let resolve!: () => void
  const promise = new Promise<void>((done) => { resolve = done })
  return { promise, resolve }
}

let host: HTMLDivElement
let root: Root
const mount = async (node: React.ReactNode) => {
  host = document.createElement('div')
  document.body.append(host)
  root = createRoot(host)
  await act(() => root.render(node))
}
const click = async (el: Element | null) => {
  expect(el).toBeTruthy()
  await act(() => { (el as HTMLElement).click() })
}
const byText = (text: string) => [...document.body.querySelectorAll('button')].find((b) => b.textContent?.includes(text)) ?? null

afterEach(async () => {
  await act(() => root?.unmount())
  host?.remove()
  delete document.documentElement.dataset.offline
  vi.useRealTimers()
})

describe('ActionSheet', () => {
  it('khoá tấm và không đóng cho tới khi lời hứa xong', async () => {
    const gate = deferred()
    const onSelect = vi.fn(() => gate.promise)
    const onClose = vi.fn()
    await mount(<ActionSheet open onClose={onClose} title="Nocturne Stories" actions={[{ label: 'Chọn video nổi trội', onSelect }]} />)
    const row = byText('Chọn video nổi trội')
    await click(row)
    expect(onSelect).toHaveBeenCalledTimes(1)
    expect(onClose).not.toHaveBeenCalled()
    expect((row as HTMLButtonElement).disabled).toBe(true)
    // bấm nền mờ trong lúc chờ cũng không đóng
    await click(document.querySelector('.m-scrim'))
    expect(onClose).not.toHaveBeenCalled()
    await act(async () => { gate.resolve() })
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('báo lỗi tại chỗ và giữ tấm mở khi hành động ném lỗi', async () => {
    const onClose = vi.fn()
    await mount(<ActionSheet open onClose={onClose} title="Chọn" actions={[{ label: 'Bỏ liên kết', destructive: true, onSelect: () => Promise.reject(new Error('Máy chủ từ chối')) }]} />)
    await click(byText('Bỏ liên kết'))
    await act(async () => { await Promise.resolve() })
    expect(document.querySelector('[role="alert"]')?.textContent).toBe('Máy chủ từ chối')
    expect(onClose).not.toHaveBeenCalled()
  })

  it('mô tả phụ hiện dưới nhãn hàng', async () => {
    await mount(<ActionSheet open onClose={() => {}} title="Chọn" actions={[{ label: 'Bỏ liên kết', description: 'Hỏi xác nhận trước', onSelect: () => {} }]} />)
    expect(document.body.textContent).toContain('Hỏi xác nhận trước')
  })
})

describe('ConfirmSheet', () => {
  it('là alertdialog, chặn đóng khi đang chạy, xong thì đóng', async () => {
    const gate = deferred()
    const onSelect = vi.fn(() => gate.promise)
    const onClose = vi.fn()
    await mount(<ConfirmSheet open onClose={onClose} title="Bật tự duyệt ảnh?" description="Ảnh mới sẽ được duyệt tự động." action={{ label: 'Bật tự duyệt', onSelect }} />)
    expect(document.querySelector('[role="alertdialog"]')).toBeTruthy()
    await click(byText('Bật tự duyệt'))
    expect(onSelect).toHaveBeenCalledTimes(1)
    await click(document.querySelector('.m-scrim'))
    expect(onClose).not.toHaveBeenCalled()
    await act(async () => { gate.resolve() })
    expect(onClose).toHaveBeenCalledTimes(1)
  })
})

describe('SelectionBar', () => {
  it('ẩn thanh tab qua useTabMode("hide")', async () => {
    const Probe = () => <span data-tabmode={useMobileRuntime().tabMode} />
    await mount(<MobileRuntimeProvider><Probe /><SelectionBar count={3} /></MobileRuntimeProvider>)
    expect(document.querySelector('[data-tabmode]')?.getAttribute('data-tabmode')).toBe('hide')
    expect(document.body.textContent).toContain('Đã chọn 3')
  })
})

describe('OfflineBanner', () => {
  beforeEach(() => { vi.useFakeTimers() })

  it('chỉ hiện khi mất mạng đã bền 3 giây, rồi đếm ngược và làm mờ nội dung', async () => {
    const onRetry = vi.fn()
    await mount(<OfflineBanner offline retryIn={5} onRetry={onRetry} />)
    expect(document.querySelector('[role="status"]')).toBeNull()
    expect(document.documentElement.dataset.offline).toBeUndefined()
    await act(async () => { vi.advanceTimersByTime(2999) })
    expect(document.querySelector('[role="status"]')).toBeNull()
    await act(async () => { vi.advanceTimersByTime(1) })
    expect(document.querySelector('[role="status"]')?.textContent).toContain('thử lại sau 5 s')
    expect(document.documentElement.dataset.offline).toBe('')
    await act(async () => { vi.advanceTimersByTime(1000) })
    expect(document.querySelector('[role="status"]')?.textContent).toContain('thử lại sau 4 s')
    await click(byText('Thử ngay'))
    expect(onRetry).toHaveBeenCalledTimes(1)
  })

  it('mất mạng chập chờn chưa bền thì không hiện', async () => {
    await mount(<OfflineBanner offline onRetry={() => {}} />)
    await act(async () => { vi.advanceTimersByTime(2000) })
    await act(() => root.render(<OfflineBanner offline={false} onRetry={() => {}} />))
    await act(async () => { vi.advanceTimersByTime(5000) })
    expect(document.querySelector('[role="status"]')).toBeNull()
  })

  it('gỡ banner thì bỏ cờ làm mờ', async () => {
    await mount(<OfflineBanner offline onRetry={() => {}} />)
    await act(async () => { vi.advanceTimersByTime(3000) })
    expect(document.documentElement.dataset.offline).toBe('')
    await act(() => root.render(<OfflineBanner offline={false} onRetry={() => {}} />))
    expect(document.documentElement.dataset.offline).toBeUndefined()
  })
})

describe('Stepper', () => {
  it('kẹp trong [min, max] và khoá nút ở biên', async () => {
    const onChange = vi.fn()
    await mount(<Stepper value={5} min={5} max={7} onChange={onChange} label="Số video" />)
    const [minus, plus] = [...document.querySelectorAll('button')] as HTMLButtonElement[]
    expect(minus.disabled).toBe(true)
    expect(plus.disabled).toBe(false)
    await click(plus)
    expect(onChange).toHaveBeenLastCalledWith(6)
    await act(() => root.render(<Stepper value={7} min={5} max={7} onChange={onChange} label="Số video" />))
    expect((document.querySelectorAll('button')[1] as HTMLButtonElement).disabled).toBe(true)
    expect(document.querySelector('output')?.textContent).toBe('7')
  })

  it('không phát onChange khi đã ở biên', async () => {
    const onChange = vi.fn()
    await mount(<Stepper value={0} min={0} max={10} onChange={onChange} />)
    await click(document.querySelectorAll('button')[0])
    expect(onChange).not.toHaveBeenCalled()
  })
})

describe('ô nhập và chip điện thoại', () => {
  it('Field mobile dùng chữ 16px để iOS không phóng trang', async () => {
    await mount(<Field mobile aria-label="Tên kênh" />)
    expect(getComputedStyle(document.querySelector('input')!).fontSize).toBe('16px')
  })

  it('Chip mobile thêm vùng chạm 44px bằng ::before', async () => {
    await mount(<Chip size="mobile" label="Lọc" />)
    expect(document.body.textContent).toBe('Lọc')
  })

  it('Button block chiếm trọn bề ngang', async () => {
    await mount(<Button block size="xl">Làm video cho kênh</Button>)
    expect(document.querySelector('button')!.textContent).toContain('Làm video cho kênh')
  })
})

describe('chỉ báo', () => {
  it('StatusPill: working là vòng xoay, còn lại là chấm', async () => {
    await mount(<div><StatusPill status="working">Đang làm</StatusPill><StatusPill status="review">Chờ duyệt</StatusPill></div>)
    const pills = document.querySelectorAll('span')
    expect(pills[0].textContent).toBe('Đang làm')
    expect(document.body.textContent).toContain('Chờ duyệt')
  })

  it('MeterBar kẹp 0–100 và công bố aria-valuenow', async () => {
    await mount(<MeterBar value={1.4} label="Tiến độ" />)
    expect(document.querySelector('[role="progressbar"]')!.getAttribute('aria-valuenow')).toBe('100')
  })

  it('InitialsAvatar lấy chữ đầu của tên', () => {
    expect(initialsOf('Kênh Thực Tế')).toBe('KT')
    expect(initialsOf('Nocturne')).toBe('NO')
    expect(initialsOf('  ')).toBe('')
  })

  it('VideoRow hiện tiêu đề, trạng thái và nút bên phải', async () => {
    await mount(<VideoRow title="I Took a Job Watching an Empty Mall" status={<StatusPill status="review">Chờ duyệt · 7 ảnh</StatusPill>} action={<Button>Duyệt</Button>} />)
    expect(document.body.textContent).toContain('Empty Mall')
    expect(document.body.textContent).toContain('Chờ duyệt · 7 ảnh')
    expect(byText('Duyệt')).toBeTruthy()
  })

  it('ListRow bấm được và có dòng phụ', async () => {
    const onClick = vi.fn()
    await mount(<ListRow icon="trend" title="Tự lấy video" hint="Video mới tự vào hàng chờ" onClick={onClick} />)
    expect(document.body.textContent).toContain('Video mới tự vào hàng chờ')
    await click(document.querySelector('.m-row'))
    expect(onClick).toHaveBeenCalledTimes(1)
  })
})

describe('MobilePage', () => {
  it('một vùng cuộn, thanh trên 52 và chân trang', async () => {
    await mount(<MobilePage title="Kênh Thực Tế" subtitle="Hôm nay 4/7" onBack={() => {}} footer={<Button>Lưu</Button>}>Nội dung</MobilePage>)
    expect(document.querySelectorAll('main.m-scroll').length).toBe(1)
    expect(document.querySelector('header.m-topbar')).toBeTruthy()
    expect(document.querySelector('.m-back')?.getAttribute('aria-label')).toBe('Quay lại')
    expect(document.querySelector('.m-footer')).toBeTruthy()
    expect(document.querySelector('.m-title small')?.textContent).toBe('Hôm nay 4/7')
  })

  it('có nhãn quay lại thì nút rộng theo nhãn', async () => {
    await mount(<MobilePage title="Cài đặt" backTitle="Kênh" onBack={() => {}}>Nội dung</MobilePage>)
    expect(document.querySelector('.m-back')?.textContent).toContain('Kênh')
  })
})
