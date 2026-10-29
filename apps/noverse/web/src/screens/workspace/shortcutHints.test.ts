import { describe, expect, it } from 'vitest'
import { t } from '../../i18n/vi'
import { splitHint } from './Composer'

describe('gợi ý phím tắt dưới ô nhập', () => {
  it('phím có dấu cách "[ ]" nằm trọn trong một ô phím (Main.dc.html)', () => {
    expect(splitHint(t.shortcuts.panels)).toEqual(['[ ]', 'bảng bên'])
  })

  it('các phím một token vẫn tách đúng', () => {
    expect(splitHint(t.shortcuts.newline)).toEqual(['Ctrl+J', 'xuống dòng'])
    expect(splitHint(t.shortcuts.command)).toEqual(['/', 'lệnh'])
    expect(splitHint(t.shortcuts.send)).toEqual(['Enter', 'gửi'])
    expect(splitHint(t.shortcuts.palette)).toEqual(['⌘K', 'bảng lệnh'])
  })
})
