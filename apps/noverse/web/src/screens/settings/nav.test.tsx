import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import { SettingsNav } from './nav'

vi.mock('@noverse/ui', async (importOriginal) => ({ ...(await importOriginal<object>()),  Icon: ({ name }: { name: string }) => <i data-icon={name} /> }))
const session = vi.hoisted(() => ({ pendingLessons: 0 }))
vi.mock('../../api/session', () => ({ useSession: () => session }))

const render = (admin: boolean, dots: Parameters<typeof SettingsNav>[0]['dots'] = {}, rulesCount?: number) =>
  renderToStaticMarkup(<SettingsNav current="rules" scope="story" admin={admin} rulesCount={rulesCount} dots={dots} onPick={() => {}} />)

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
    const order = ['Não hệ thống', 'Thành viên', 'Kết nối AI', 'Xuất tự động', 'Báo động máy chủ', 'Giao diện', 'Nâng cao'].map((label) => html.indexOf(label))
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
    expect(four).toMatch(/var\(--sysbrain-count-bg\)[^>]*>4</)
    expect(render(false)).not.toContain('Não hệ thống')
    session.pendingLessons = 0
  })
  it('chấm tím chỉ ở mục có ghi đè và không đè lên số của Quy tắc viết', () => {
    const dotted = render(false, { writing: true, rules: true }, 7)
    expect(dotted.match(/border-radius:50%/g)).toHaveLength(1)
    expect(dotted).toContain('>7<')
  })
  it('tên mục Thông tin đổi theo cấp', () => {
    expect(render(false)).toContain('Thông tin truyện')
    expect(renderToStaticMarkup(<SettingsNav current="rules" scope="project" admin={false} dots={{}} onPick={() => {}} />)).toContain('Thông tin dự án')
  })
})
