// @vitest-environment happy-dom
// Kit additions cho nhánh thứ hai (Button quiet · Banner agent · toast thẻ · orb theo trợ lý).
import { act } from 'react'
import { createRoot, type Root } from 'react-dom/client'
import { renderToStaticMarkup } from 'react-dom/server'
import * as stylex from '@stylexjs/stylex'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { Button } from './Button'
import { Banner } from './Banner'
import { ConfirmDialog } from './ConfirmDialog'
import { dismissToast, toast, ToastHost } from './Toast'

// Vitest extracts StyleX CSS without attaching it to happy-dom. So sánh atom đã biên
// dịch với hợp đồng ngữ nghĩa (như Selects.test.tsx), không so class hash trực tiếp.
const cardContract = stylex.create({
  card: { display: 'flex', flexDirection: 'column', width: 300, borderRadius: 14 },
})
const pillContract = stylex.create({
  pill: { borderRadius: 999, minHeight: 40 },
})

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })
const roots: Root[] = []

afterEach(async () => {
  for (const root of roots.splice(0)) await act(() => root.unmount())
  document.body.replaceChildren()
  vi.useRealTimers(); vi.unstubAllGlobals()
})

async function mountToast() {
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)
  roots.push(root)
  await act(async () => { root.render(<ToastHost />) })
  return container
}


it('Button variant="quiet" render như button thường, không viền (chữ mờ, hover sáng hơn)', () => {
  const html = renderToStaticMarkup(<Button variant="quiet">Bỏ qua</Button>)
  expect(html).toContain('<button')
  expect(html).toContain('Bỏ qua')
})

it('Button size="compact" đúng hợp đồng 32/r8/13 của artboard Settings', () => {
  const contract = stylex.create({
    compact: { height: 32, paddingInline: 12, borderRadius: 8, fontSize: 13 },
  })
  const html = renderToStaticMarkup(<Button size="compact">Kiểm tra lại</Button>)
  const cls = stylex.props(contract.compact).className!.split(' ').filter(Boolean)
  for (const atom of cls) expect(html).toContain(atom)
})

it('Button size="compact" không đè fw 600 của primary/danger nhưng vẫn cho ghost 400', () => {
  const probe = stylex.create({ w600: { fontWeight: 600 }, w400: { fontWeight: 400 } })
  const atom600 = stylex.props(probe.w600).className!
  const atom400 = stylex.props(probe.w400).className!
  const atoms = (html: string) => new Set(html.match(/class="([^"]+)"/)![1].split(' '))

  const primaryCompact = atoms(renderToStaticMarkup(<Button variant="primary" size="compact">Lưu</Button>))
  expect(primaryCompact.has(atom600)).toBe(true)
  expect(primaryCompact.has(atom400)).toBe(false)

  const dangerCompact = atoms(renderToStaticMarkup(<Button variant="danger" size="compact">Xoá</Button>))
  expect(dangerCompact.has(atom600)).toBe(true)

  for (const variant of ['ghost', 'quiet', 'warnGhost'] as const) {
    const compact = atoms(renderToStaticMarkup(<Button variant={variant} size="compact">Huỷ</Button>))
    expect(compact.has(atom400)).toBe(true)
    expect(compact.has(atom600)).toBe(false)
  }
})

it('Button variant="warnGhost" dùng viền .45 và chữ ghost-ink (nền trong)', () => {
  const contract = stylex.create({
    warnGhost: { backgroundColor: 'transparent', color: 'var(--kit-err-ghost-ink)', borderColor: 'var(--kit-errb-45)' },
  })
  const html = renderToStaticMarkup(<Button variant="warnGhost" size="compact">Xoá nhóm</Button>)
  const atoms = stylex.props(contract.warnGhost).className!.split(' ').filter(Boolean)
  expect(atoms.length).toBeGreaterThan(0)
  for (const atom of atoms) expect(html).toContain(atom)
})

it('ConfirmDialog size="compact" áp cỡ 32 cho cả nút huỷ lẫn nút chính', () => {
  const html = renderToStaticMarkup(
    <ConfirmDialog inline size="compact" title="Bỏ nhà cung cấp?" body="Khoá sẽ bị xoá khỏi máy này." confirmLabel="Bỏ" onConfirm={() => {}} onClose={() => {}} />,
  )
  const contract = stylex.create({ compact: { height: 32, paddingInline: 12, borderRadius: 8, fontSize: 13 } })
  const atoms = stylex.props(contract.compact).className!.split(' ').filter(Boolean)
  for (const atom of atoms) expect(html).toContain(atom)
  // Mặc định (không truyền size) vẫn là 36/r10 của artboard Dialogs.
  const def = renderToStaticMarkup(<ConfirmDialog inline title="Xoá?" body="x" confirmLabel="Xoá" onConfirm={() => {}} onClose={() => {}} />)
  expect(def).not.toContain(atoms[0])
})

it('Button size="xl" đúng hợp đồng CTA điện thoại 48/r13/15', () => {
  const contract = stylex.create({
    xl: { height: 48, paddingInline: 16, borderRadius: 13, fontSize: 15, fontWeight: 600, gap: 8 },
  })
  const html = renderToStaticMarkup(<Button variant="primary" size="xl">Tạo kênh</Button>)
  const cls = stylex.props(contract.xl).className!.split(' ').filter(Boolean)
  for (const atom of cls) expect(html).toContain(atom)
})

it('Banner tone="agent" hiện orb của trợ lý và màu quầng của trợ lý đó', () => {
  const html = renderToStaticMarkup(<Banner tone="agent" agent="editor" title="Hoạ sĩ giải thích" action={{ label: 'Xem', onClick: () => {} }}>Đề xuất thay nền cho cảnh mở đầu.</Banner>)
  expect(html).toContain('--banner-agent-editor-bg')
  expect(html).toContain('role="status"')
  expect(html).toContain('Hoạ sĩ giải thích')
  // Orb là span tròn gradient, không phải icon trạng thái.
  expect(html).not.toContain('info-circle')
})

it('Banner tone="agent" của trợ lý khác dùng đúng quầng trợ lý đó', () => {
  const html = renderToStaticMarkup(<Banner tone="agent" agent="arbiter" title="Kiểm định nhắc" action={{ label: 'Mở', onClick: () => {} }} />)
  expect(html).toContain('--banner-agent-arbiter-bg')
})

it('ConfirmDialog agent=trợ lý render orb của trợ lý đó; agent=true giữ orb writer', () => {
  const editor = renderToStaticMarkup(<ConfirmDialog inline agent="editor" title="Tạm dừng Hoạ sĩ?" body="Video đang vẽ sẽ chờ." confirmLabel="Tạm dừng" onConfirm={() => {}} onClose={() => {}} />)
  expect(editor).toContain('--orb-editor')
  const legacy = renderToStaticMarkup(<ConfirmDialog inline agent title="Chạy tiếp?" body="x" confirmLabel="Chạy" onConfirm={() => {}} onClose={() => {}} />)
  expect(legacy).toContain('--orb-writer')
})

describe('toast dạng thẻ', () => {
  it('thẻ 300px có tiêu đề video, đúng một nút việc, role="alert", ở lại khi hết giờ', () => {
    vi.useFakeTimers()
    vi.stubGlobal('window', { matchMedia: () => ({ matches: false }) })
    const id = toast.card({ title: 'Duyệt ảnh cho “Mưa trên phố đèn”', text: 'Hoạ sĩ vẽ xong 4 phương án.', action: { label: 'Duyệt ảnh', onClick: () => {} } })
    const first = renderToStaticMarkup(<ToastHost />)
    expect(first).toContain('role="alert"')
    expect(first).toContain('Duyệt ảnh cho “Mưa trên phố đèn”')
    expect(first.match(/<button/g)?.length).toBe(2) // nút việc + nút đóng
    vi.advanceTimersByTime(60_000)
    expect(renderToStaticMarkup(<ToastHost />)).toContain('Mưa trên phố đèn')
    dismissToast(id)
    vi.advanceTimersByTime(600)
    expect(renderToStaticMarkup(<ToastHost />)).not.toContain('Mưa trên phố đèn')
  })
  it('thẻ dùng layout card (300px, cột, bo 14) chứ không phải pill ngang', async () => {
    // happy-dom đã có matchMedia thật — không stub như bản render static.
    const container = await mountToast()
    await act(async () => { toast.card({ title: 'Duyệt ảnh', action: { label: 'Mở', onClick: () => {} } }) })
    const alert = container.querySelector('[role="alert"]') as HTMLElement
    expect(alert).toBeTruthy()
    const cls = alert.className.split(' ').filter(Boolean)
    const cardCls = (stylex.props(cardContract.card).className ?? '').split(' ').filter(Boolean)
    const pillCls = (stylex.props(pillContract.pill).className ?? '').split(' ').filter(Boolean)
    expect(cardCls.length).toBeGreaterThan(0)
    // Atom của layout card phải có trên khung; atom riêng của pill (bo 999, min-height 40) phải vắng mặt.
    for (const name of cardCls) expect(cls).toContain(name)
    for (const name of pillCls) expect(cls).not.toContain(name)
  })
})
