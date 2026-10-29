// G13: dòng `/…` trong ô chat web — lệnh nào do giao diện xử lý, lệnh nào gửi server.
import { describe, expect, it } from 'vitest'
import { helpPath, slashRoute } from './slash'

describe('slashRoute', () => {
  it('/help mở trang Hướng dẫn lệnh, không gửi server', () => {
    expect(slashRoute('/help')).toEqual({ kind: 'help' })
    expect(slashRoute('  /help  ')).toEqual({ kind: 'help' })
    expect(helpPath()).toBe('/help/commands')
    expect(helpPath('export')).toBe('/help/commands/export')
  })

  it('/reopen và /import giữ hộp thoại sẵn có', () => {
    expect(slashRoute('/reopen viết tiếp')).toEqual({ kind: 'dialog', dialog: 'reopen' })
    expect(slashRoute('/import ban-thao.md --yes')).toEqual({ kind: 'dialog', dialog: 'import' })
  })

  it('/cocreate và alias /plan giữ luồng đồng sáng tác', () => {
    expect(slashRoute('/cocreate')).toEqual({ kind: 'cocreate' })
    expect(slashRoute('/plan giai đoạn 2')).toEqual({ kind: 'cocreate' })
  })

  it('còn lại gửi POST …/command', () => {
    expect(slashRoute('/export "a b.epub" from=1')).toEqual({ kind: 'server' })
    expect(slashRoute('/next')).toEqual({ kind: 'server' })
    expect(slashRoute('/review on')).toEqual({ kind: 'server' })
    expect(slashRoute('/model writer')).toEqual({ kind: 'server' })
  })
})
