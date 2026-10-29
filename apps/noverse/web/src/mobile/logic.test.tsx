import { afterEach, describe, expect, it, vi } from 'vitest'
import { renderToStaticMarkup } from 'react-dom/server'
import type { CommandSpec } from '../api/types'
import { useMobileLayout } from './index'
import { mobileCommands, pickToastBottom, shouldCloseSheet, toastBottomFor } from './logic'

afterEach(() => vi.unstubAllGlobals())

describe('khung điện thoại', () => {
  it('chỉ đóng sheet khi kéo xuống hơn 30% chiều cao', () => {
    expect(shouldCloseSheet(130, 440)).toBe(false)
    expect(shouldCloseSheet(132, 440)).toBe(false)
    expect(shouldCloseSheet(133, 440)).toBe(true)
    expect(shouldCloseSheet(-200, 440)).toBe(false)
    expect(shouldCloseSheet(1, 0)).toBe(false)
  })

  it('layout thực dùng matchMedia: 640px là điện thoại, 641px và desktop giữ bản lớn', () => {
    let width = 640
    vi.stubGlobal('matchMedia', (query: string) => ({ matches: width <= Number(/max-width:\s*(\d+)px/.exec(query)?.[1]), addEventListener() {}, removeEventListener() {} }))
    function Probe() { return <span>{useMobileLayout() ? 'điện thoại' : 'máy tính'}</span> }
    expect(renderToStaticMarkup(<Probe />)).toContain('điện thoại')
    width = 641
    expect(renderToStaticMarkup(<Probe />)).toContain('máy tính')
    width = 1440
    expect(renderToStaticMarkup(<Probe />)).toContain('máy tính')
  })

  it('sheet lệnh ẩn config/start/simulate/importsim trước cả tìm kiếm và chọn tham số', () => {
    const names = ['config', 'start', 'simulate', 'importsim', 'next', 'diag', 'export', 'review', 'help', 'model']
    const specs = names.map((name): CommandSpec => ({ name, aliases: [], usage: `/${name}`, group: 'write', autoExecute: false, needsIdle: false, params: [] }))
    expect(mobileCommands(specs).map((spec) => spec.name)).toEqual(['next', 'diag', 'export', 'review', 'help', 'model'])
    expect(specs).toHaveLength(10)
  })

  it('toast đặt bottom theo từng artboard; màn tự đặt (ưu tiên cao) thắng mặc định theo route', () => {
    expect(['workspace', 'reader', 'audio', 'project', 'command', 'activity', 'notifications'].map(toastBottomFor)).toEqual([214, 100, 168, 100, 112, 30, 30])
    expect(toastBottomFor('login')).toBeNull()
    expect(pickToastBottom([])).toBeNull()
    expect(pickToastBottom([{ px: 30, priority: 0 }, { px: 760, priority: 1 }, { px: 100, priority: 0 }])).toBe(760)
    expect(pickToastBottom([{ px: 100, priority: 0 }, { px: 30, priority: 0 }])).toBe(30)
  })
})
