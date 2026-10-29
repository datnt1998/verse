// brief 25/10 › câu hỏi treo `/reopen`: người không có quyền viết không thấy lệnh trong gợi ý `/`
// (và ⌘K); gõ tay thì thẻ lỗi “Bạn chỉ có quyền đọc truyện này”.
import { describe, expect, it } from 'vitest'
import { COMMANDS } from '../../mock/data'
import { CMD_CHECK } from '../i18n/commands.vi'
import { buildSlashView, canRunCommand, slashContext, WRITE_COMMANDS } from './model'
import { dismissSlash, slashDismissed } from './store'

const spec = (name: string) => {
  const found = COMMANDS.find((entry) => entry.name === name)
  if (!found) throw new Error(`missing mock spec ${name}`)
  return found
}
const ctx = (writable: boolean, dismissed = false) => slashContext(
  { bookTitle: 'Truyện', completedCount: 12, isRunning: false } as never, COMMANDS, {}, { writable, dismissed },
)

describe('quyền viết của lệnh', () => {
  it('chỉ `/reopen` cần quyền viết', () => {
    expect(WRITE_COMMANDS).toEqual(['reopen'])
    expect(canRunCommand(spec('reopen'), false)).toBe(false)
    expect(canRunCommand(spec('reopen'), true)).toBe(true)
    expect(canRunCommand(spec('help'), false)).toBe(true)
  })

  it('danh sách `/` ẩn `/reopen` khi chỉ có quyền đọc, vẫn còn các lệnh khác', () => {
    const list = buildSlashView('/', ctx(false))
    const keys = list?.items.map((item) => item.key) ?? []
    expect(keys).not.toContain('reopen')
    expect(keys).toContain('help')
    expect(buildSlashView('/', ctx(true))?.items.map((item) => item.key)).toContain('reopen')
  })

  it('gõ tay `/reopen` khi chỉ đọc → lỗi quyền, không cho chạy', () => {
    const view = buildSlashView('/reopen', ctx(false))
    expect(view?.mode).toBe('args')
    expect(view?.hasError).toBe(true)
    expect(view?.detail.status?.text).toBe(CMD_CHECK.readOnly)
    expect(view?.submitLine).toBeNull()
    const allowed = buildSlashView('/reopen', ctx(true))
    expect(allowed?.mode).toBe('list')
    expect(allowed?.hasError).toBe(false)
    expect(buildSlashView('/reopen ', ctx(true))?.submitLine).toBe('/reopen')
  })

  it('Esc đã đóng popover thì không dựng lại view cho đúng dòng đó', () => {
    expect(buildSlashView('/', ctx(true, true))).toBeNull()
  })

  it('Esc đóng gợi ý nhưng giữ chữ; gõ tiếp một ký tự là gợi ý hiện lại (câu 24)', () => {
    dismissSlash('/')
    expect(slashDismissed('/')).toBe(true)
    expect(buildSlashView('/', ctx(true, slashDismissed('/')))).toBeNull()
    // Chữ đã gõ vẫn còn nguyên dòng “/”; gõ thêm “e” thành dòng khác → mở lại.
    expect(slashDismissed('/e')).toBe(false)
    const reopened = buildSlashView('/e', ctx(true, slashDismissed('/e')))
    expect(reopened?.mode).toBe('list')
    expect(reopened?.items.map((item) => item.key)).toContain('export')
  })
})
