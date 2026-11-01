import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import { SettingsNav } from './nav'

vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()),  Icon: ({ name }: { name: string }) => <i data-icon={name} /> }))
const session = vi.hoisted(() => ({ pendingLessons: 0 }))
vi.mock('../../api/session', () => ({ useSession: () => session }))

const render = (admin: boolean, dots: Parameters<typeof SettingsNav>[0]['dots'] = {}) =>
  renderToStaticMarkup(<SettingsNav current="models" scope="story" admin={admin} dots={dots} onPick={() => {}} />)

describe('menu trái Tuỳ chỉnh', () => {
  it('người không phải quản trị không thấy nhóm HỆ THỐNG · QUẢN TRỊ lẫn các mục của nó', () => {
    const html = render(false)
    expect(html).toContain('SÁNG TÁC')
    for (const label of ['Thành viên', 'HỆ THỐNG', 'Kết nối AI', 'Xuất tự động', 'Báo động máy chủ', 'Giao diện', 'Nâng cao']) expect(html).not.toContain(label)
  })
  it('quản trị thấy nhóm có vương miện: Não hệ thống đứng đầu rồi năm mục theo thứ tự artboard', () => {
    const html = render(true)
    expect(html).toContain('HỆ THỐNG · QUẢN TRỊ')
    expect(html).toContain('data-icon="crown"')
    const order = ['Não hệ thống', 'Thành viên', 'Lưu trữ tệp', 'Kết nối AI', 'Xuất tự động', 'Báo động máy chủ', 'Giao diện', 'Nâng cao'].map((label) => html.indexOf(label))
    expect(order.every((at) => at > 0)).toBe(true)
    expect([...order].sort((a, b) => a - b)).toEqual(order)
  })
  it('hàng Não hệ thống mở #/system/brain, kèm số bài học chờ duyệt khi có (D15)', () => {
    session.pendingLessons = 0
    const none = render(true)
    expect(none).toContain('href="#/system/brain"')
    expect(none).toContain('Bài học từ mọi truyện')
    expect(none).toContain('title="Bài học rút ra từ mọi truyện"')
    expect(none).not.toContain('rgba(201,166,245,.18)')
    session.pendingLessons = 4
    const four = render(true)
    expect(four).toContain('title="Bài học rút ra từ mọi truyện · 4 bài học chờ bạn duyệt"')
    expect(four).toContain('4 bài học chờ duyệt</span>')
    expect(four).not.toContain('var(--sysbrain-count-bg)')
    expect(render(false)).not.toContain('Não hệ thống')
    session.pendingLessons = 0
  })
  it('chấm tím ở mục có ghi đè', () => {
    const dotted = render(false, { run: true })
    expect(dotted.match(/border-radius:50%/g)).toHaveLength(1)
  })
  it('Thông tin dự án luôn hiện ở mọi cấp', () => {
    expect(render(false)).toContain('Thông tin dự án')
    expect(renderToStaticMarkup(<SettingsNav current="models" scope="project" admin={false} dots={{}} onPick={() => {}} />)).toContain('Thông tin dự án')
  })
})
