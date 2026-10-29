import { describe, expect, it } from 'vitest'
import {
  commandValue, configPathValue, contextWindowValue, dirsValue, humanDuration, idleSummary, importModelsValue, importRoleModel, isDuration, lanView,
  parseContextWindow, recoveryFile, recoveryValue, rulesPathValue, shortDate, slugWordsValue,
} from './logic'

// Ngày dựng theo giờ máy để kết quả không lệch múi giờ.
const iso = (month: number, day: number) => new Date(2026, month - 1, day, 12).toISOString()

describe('Nâng cao › Vị trí lưu: đường dẫn rút gọn như artboard', () => {
  it('tệp cấu hình: ~ ở toàn cục, <dự án>/… ở dự án, kich_ban/<truyện>/… ở truyện', () => {
    expect(configPathValue('/Users/dat/.ainovel/config.json', 'global')).toBe('~/.ainovel/config.json')
    expect(configPathValue('dit-dang-audio/.ainovel/config.json', 'project')).toBe('dit-dang-audio/.ainovel/config.json')
    expect(configPathValue('dit-dang-audio/kich_ban/01-tra-xanh/.ainovel/config.json', 'story')).toBe('kich_ban/01-tra-xanh/.ainovel/config.json')
  })
  it('thư mục quy tắc: ~/.ainovel/rules/ ở toàn cục, …/.ainovel/rules/ ở cấp dưới', () => {
    expect(rulesPathValue('/home/dat/.ainovel/rules', 'global')).toBe('~/.ainovel/rules/')
    expect(rulesPathValue('/lib/p/kich_ban/s/.ainovel/rules', 'story')).toBe('…/.ainovel/rules/')
  })
})

describe('Nâng cao › Truy cập & khôi phục', () => {
  it('shortDate: dd/mm có số 0 đầu, ISO hỏng → rỗng', () => {
    expect(shortDate(iso(10, 24))).toBe('24/10')
    expect(shortDate(iso(3, 5))).toBe('05/03')
    expect(shortDate('không phải ngày')).toBe('')
  })

  it('recoveryValue: “Đã tạo dd/mm · chưa dùng”, đã dùng, chưa từng tạo', () => {
    expect(recoveryValue({ createdAt: iso(10, 24), used: false })).toBe('Đã tạo 24/10 · chưa dùng')
    expect(recoveryValue({ createdAt: iso(10, 24), used: true })).toBe('Đã tạo 24/10 · đã dùng')
    expect(recoveryValue({ createdAt: null, used: false })).toBe('Chưa tạo mã')
  })

  it('lanView: nhãn nút theo trạng thái, chỉ khoá (kèm lý do) khi không bật tắt được', () => {
    expect(lanView(true, true)).toEqual({ value: 'Đang bật', action: 'Tắt', disabledReason: '' })
    expect(lanView(false, true)).toEqual({ value: 'Đang tắt', action: 'Bật', disabledReason: '' })
    expect(lanView(true, false).disabledReason).toBe('Chỉ bật hoặc tắt được khi dùng ngay trên máy chủ')
  })

  it('recoveryFile: tệp .txt chứa đúng mã, không chứa dòng lệnh', () => {
    const file = recoveryFile('TRAX-8K2M-QP7D-41ZN')
    expect(file.name).toBe('ma-khoi-phuc-noverse.txt')
    expect(file.text).toContain('Mã khôi phục của quản trị Noverse')
    expect(file.text).toContain('TRAX-8K2M-QP7D-41ZN')
    expect(file.text).not.toContain('ainovel-cli')
    expect(file.text).not.toMatch(/ainovel/i)
  })
})

describe('Nâng cao › Tinh chỉnh model & tên thư mục', () => {
  it('humanDuration: giây, phút, giờ; không đọc được thì giữ nguyên', () => {
    expect(humanDuration('90s')).toBe('90 giây')
    expect(humanDuration('5m')).toBe('5 phút')
    expect(humanDuration('1h30m')).toBe('90 phút')
    expect(humanDuration('2h')).toBe('2 giờ')
    expect(humanDuration('1m30s')).toBe('90 giây')
    expect(humanDuration('vài phút')).toBe('vài phút')
  })

  it('isDuration: chỉ nhận dạng Go h/m/s hoặc rỗng', () => {
    expect(isDuration('')).toBe(true)
    expect(isDuration('900s')).toBe(true)
    expect(isDuration('1h30m')).toBe(true)
    expect(isDuration('90')).toBe(false)
    expect(isDuration('5 phút')).toBe(false)
  })

  it('idleSummary: mặc định, một giá trị chung, khác nhau theo nhà cung cấp', () => {
    expect(idleSummary([])).toBe('Mặc định (5 phút)')
    expect(idleSummary(['', undefined])).toBe('Mặc định (5 phút)')
    expect(idleSummary(['90s', '90s'])).toBe('90 giây')
    expect(idleSummary(['90s', ''])).toBe('Khác nhau theo nhà cung cấp')
  })

  it('contextWindowValue: dùng formatter K/M chung, rơi về mặc định máy chủ', () => {
    expect(contextWindowValue(200000, 1)).toBe('200K token')
    expect(contextWindowValue(undefined, 128000)).toBe('128K token (mặc định)')
    expect(contextWindowValue(undefined, undefined)).toBe('Mặc định của máy chủ')
  })

  it('parseContextWindow: rỗng xoá khoá, số nguyên dương nhận, còn lại báo lỗi', () => {
    expect(parseContextWindow(' ')).toBeUndefined()
    expect(parseContextWindow('200000')).toBe(200000)
    expect(parseContextWindow('0')).toBeNull()
    expect(parseContextWindow('20k')).toBeNull()
    expect(parseContextWindow('-5')).toBeNull()
  })

  it('importModelsValue / importRoleModel: cần đủ provider + model mới tính là đặt riêng', () => {
    expect(importModelsValue(undefined)).toBe('Theo Mặc định (3 bước)')
    expect(importModelsValue({ import_segment: { provider: 'a', model: '' }, import_analyze: { reasoning_effort: 'low' } } as never)).toBe('Theo Mặc định (3 bước)')
    expect(importModelsValue({ import_segment: { provider: 'a', model: 'm' } })).toBe('1/3 bước đặt riêng')
    expect(importRoleModel({ import_analyze: { provider: 'p', model: 'q' } }, 'import_analyze')).toBe('p/q')
  })

  it('dirsValue / slugWordsValue / commandValue: đúng chuỗi artboard', () => {
    expect(dirsValue({ ideas: 'y-tuong', assets: 'tai-san', stories: 'kich_ban', exports: 'xuat-ban' })).toBe('y-tuong · tai-san · kich_ban · xuat-ban')
    expect(slugWordsValue(5)).toBe('5 từ đầu, bỏ dấu')
    expect(commandValue(undefined)).toBe('chưa đặt — dùng thông báo hệ thống')
    expect(commandValue('  say "$NOTIFY_TITLE" ')).toBe('say "$NOTIFY_TITLE"')
  })
})
