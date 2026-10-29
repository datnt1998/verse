import { beforeEach, describe, expect, it } from 'vitest'
import { COMMANDS } from '../../mock/data'
import {
  acceptSlash, buildSlashView, checkLine, composeLine, filterCommands, matchesQuery, norm,
  parseLine, pushRecentCommand, quoteIfNeeded, readRecentCommands, slashContext, splitArgs, suggestCommands, tokenize,
  valuesOf, type SlashContext,
} from './model'

const spec = (name: string) => {
  const found = COMMANDS.find((entry) => entry.name === name)
  if (!found) throw new Error(`missing mock spec ${name}`)
  return found
}
const ctx = (over: Partial<SlashContext> = {}): SlashContext => ({
  specs: COMMANDS, completedCount: 5, bookTitle: 'Trà Xanh', isRunning: false,
  writingChapter: 6, recent: [], ...over,
})

describe('tokenize (G12: ngoặc kép/đơn)', () => {
  it('tách theo khoảng trắng, giữ nguyên token thường', () => {
    expect(tokenize('from=1 to=30')).toEqual([
      { value: 'from=1', quoted: false },
      { value: 'to=30', quoted: false },
    ])
  })

  it('gom phần trong ngoặc kép, kể cả dính tiền tố --guide=', () => {
    expect(tokenize('nhap.txt --guide="Hồi thứ X cũng là chương"')).toEqual([
      { value: 'nhap.txt', quoted: false },
      { value: '--guide=Hồi thứ X cũng là chương', quoted: true },
    ])
  })

  it('nhận ngoặc đơn và bỏ ngoặc khi trả giá trị', () => {
    expect(tokenize("'a b.txt'")).toEqual([{ value: 'a b.txt', quoted: true }])
  })

  it('ngoặc không đóng thì lấy hết phần còn lại', () => {
    expect(tokenize('"abc def')).toEqual([{ value: 'abc def', quoted: true }])
  })
})

describe('splitArgs / quoteIfNeeded — khớp commands.SplitArgs (parse.go)', () => {
  it('trong ngoặc, \\" và \\\\ là ký tự thoát', () => {
    expect(tokenize('"a \\"b\\" c\\\\d"')).toEqual([{ value: 'a "b" c\\d', quoted: true }])
    expect(tokenize("'it\\'s'")).toEqual([{ value: "it's", quoted: true }])
  })

  it('ngoài ngoặc, dấu \\ giữ nguyên (đường dẫn Windows); trong ngoặc \\ + ký tự khác cũng giữ nguyên', () => {
    expect(tokenize('C:\\truyen\\a.txt')).toEqual([{ value: 'C:\\truyen\\a.txt', quoted: false }])
    expect(tokenize('"C:\\truyen\\a.txt"')).toEqual([{ value: 'C:\\truyen\\a.txt', quoted: true }])
  })

  it('ngoặc không đóng → unterminated (kể cả \\" sát cuối dòng)', () => {
    expect(splitArgs('"abc def').unterminated).toBe(true)
    expect(splitArgs('"abc \\"').unterminated).toBe(true)
    expect(splitArgs('"abc def"').unterminated).toBe(false)
  })

  it('lệnh có ngoặc không đóng → báo lỗi “Cách dùng: …” như máy chủ, không cho chạy', () => {
    const parsed = parseLine('/import "a b', COMMANDS)
    expect(parsed.mode === 'args' && parsed.unterminated).toBe(true)
    const view = buildSlashView('/import "a b', ctx())
    expect(view?.hasError).toBe(true)
    expect(view?.submitLine).toBeNull()
    expect(view?.detail.status?.text).toBe(`Cách dùng: ${spec('import').usage}`)
  })

  it('quoteIfNeeded thoát " và \\ rồi splitArgs đọc lại đúng một token', () => {
    for (const value of ['a b', 'say "hi" now', 'C:\\my dir\\a b.txt', "it's", 'a"b', 'tail\\ x']) {
      const out = quoteIfNeeded(value)
      expect(splitArgs(out)).toEqual({ tokens: [{ value, quoted: true }], unterminated: false })
    }
    expect(quoteIfNeeded('plain.txt')).toBe('plain.txt')
    expect(quoteIfNeeded('C:\\a\\b.txt')).toBe('C:\\a\\b.txt')
    expect(quoteIfNeeded('"đã bọc sẵn.txt"')).toBe('"đã bọc sẵn.txt"')
  })
})

describe('parseLine', () => {
  it('không phải lệnh khi không bắt đầu bằng /', () => {
    expect(parseLine('xin chào', COMMANDS)).toEqual({ mode: 'none' })
  })

  it('đang gõ tên lệnh → list', () => {
    expect(parseLine('/ex', COMMANDS)).toEqual({ mode: 'list', q: 'ex' })
  })

  it('có khoảng trắng → args, kể cả khi chưa gõ tham số', () => {
    const parsed = parseLine('/export ', COMMANDS)
    expect(parsed.mode).toBe('args')
    if (parsed.mode !== 'args') return
    expect(parsed.spec.name).toBe('export')
    expect(parsed.tokens).toEqual([])
    expect(parsed.current).toBe('')
  })

  it('dấu cách cuối mở token rỗng: current = "" và token trước đã “xong”', () => {
    const parsed = parseLine('/export exports/tra-xanh.epub ', COMMANDS)
    expect(parsed.mode === 'args' && parsed.current).toBe('')
    const view = buildSlashView('/export exports/tra-xanh.epub ', ctx({ recentPaths: { export: ['exports/tra-xanh.epub'] } }))
    expect(view?.items.map((item) => item.token)).toEqual(['from=', 'to=', 'format=', '--overwrite', '--keep', 'doc-so'])
  })

  it('gõ dở tên khác (/pl → /plan): không ghost vì Tab điền tên chính /cocreate (artboard)', () => {
    expect(buildSlashView('/pl', ctx())?.ghost).toBe('')
    expect(buildSlashView('/plan ', ctx())?.title).toBe('Tuỳ chọn cho /cocreate')
  })

  it('nhận tên khác /plan', () => {
    const parsed = parseLine('/plan ', COMMANDS)
    expect(parsed.mode).toBe('args')
    if (parsed.mode !== 'args') return
    expect(parsed.spec.name).toBe('cocreate')
  })

  it('tên lạ + tham số → unknown', () => {
    expect(parseLine('/exprt now', COMMANDS)).toEqual({ mode: 'unknown', name: 'exprt', rest: 'now' })
  })
})

describe('valuesOf', () => {
  it('đọc vị trí, tiền tố và công tắc', () => {
    const parsed = valuesOf(spec('export'), tokenize('exports/a.epub from=3 to=7 --overwrite'))
    expect(parsed.values).toEqual({ path: 'exports/a.epub', from: '3', to: '7', format: '', ow: true, keep: false, docso: false })
    expect(parsed.extra).toBe('')
  })

  it('--guide= nhận phần chữ còn lại và giữ cờ quoted', () => {
    const parsed = valuesOf(spec('import'), tokenize('nhap.txt --guide="Hồi thứ X cũng là chương"'))
    expect(parsed.values.guide).toBe('Hồi thứ X cũng là chương')
    expect(parsed.quoted.guide).toBe(true)
  })

  it('token thừa được giữ riêng để báo lỗi', () => {
    const parsed = valuesOf(spec('help'), tokenize('abc'))
    expect(parsed.extra).toBe('abc')
  })
})

describe('khớp lệnh', () => {
  it('tên bắt đầu bằng xếp trước', () => {
    expect(filterCommands(COMMANDS, 'ex').map((entry) => entry.name)[0]).toBe('export')
  })

  it('khớp không dấu với từ khoá tiếng Việt', () => {
    expect(filterCommands(COMMANDS, 'xuat').map((entry) => entry.name)[0]).toBe('export')
    expect(filterCommands(COMMANDS, 'doi').map((entry) => entry.name)[0]).toBe('model')
    expect(filterCommands(COMMANDS, 'duyet').map((entry) => entry.name)).toContain('review')
  })

  it('gõ không dấu vẫn khớp tiêu đề có dấu', () => {
    expect(filterCommands(COMMANDS, 'truyen').map((entry) => entry.name)).toContain('export')
    expect(filterCommands(COMMANDS, 'sua').map((entry) => entry.name)).toContain('sync')
    expect(matchesQuery(spec('sync'), 'sua tay')).toBe(true)
  })

  it('gợi ý Levenshtein khi gõ sai tên', () => {
    expect(suggestCommands(COMMANDS, 'exprt').map((entry) => entry.name).slice(0, 1)).toEqual(['export'])
    expect(suggestCommands(COMMANDS, 'improt').map((entry) => entry.name).slice(0, 1)).toEqual(['import'])
  })

  it('tìm của trang Hướng dẫn khớp mọi từ, không dấu', () => {
    expect(COMMANDS.filter((entry) => matchesQuery(entry, 'xuất epub')).map((entry) => entry.name)).toEqual(['export'])
    expect(COMMANDS.filter((entry) => matchesQuery(entry, 'sửa tay')).map((entry) => entry.name)).toEqual(['sync'])
  })

  it('norm bỏ dấu và hạ chữ', () => {
    expect(norm('Đường DẪN')).toBe('duong dan')
  })
})

describe('checkLine (13-commands.md §1.3)', () => {
  const base = { completedCount: 5, bookTitle: 'Trà Xanh', isRunning: false, writingChapter: 6 }

  it('thiếu tham số bắt buộc → báo đúng nhãn tiếng Việt', () => {
    const parsed = valuesOf(spec('review'), [])
    expect(checkLine(spec('review'), parsed.values, base).err).toBe('Còn thiếu chế độ (on|off).')
  })

  it('from > to', () => {
    const parsed = valuesOf(spec('export'), tokenize('from=30 to=12'))
    expect(checkLine(spec('export'), parsed.values, base).err).toBe('khoảng chương không hợp lệ: from=30 > to=12')
  })

  it('đuôi tệp lạ → lỗi và có nút sửa .epub', () => {
    const parsed = valuesOf(spec('export'), tokenize('exports/tra-xanh.pdf'))
    const result = checkLine(spec('export'), parsed.values, base)
    expect(result.err).toContain('.pdf')
    expect(result.fixExt).toBe(true)
  })

  it('khoảng chương chưa viết xong', () => {
    const parsed = valuesOf(spec('export'), tokenize('from=6 to=30'))
    expect(checkLine(spec('export'), parsed.values, base).err).toBe('Khoảng 6..30 không có chương hoàn thành (mới xong tới chương 5).')
  })

  it('chưa tải tiến độ → không báo khoảng rỗng, không bịa “1..0”', () => {
    const parsed = valuesOf(spec('export'), tokenize(''))
    const result = checkLine(spec('export'), parsed.values, { ...base, completedCount: 0, progressUnknown: true })
    expect(result.err).toBe('')
    expect(result.infos.some((line) => line.includes('Gồm chương'))).toBe(false)
  })

  it('đường dẫn có khoảng trắng chỉ lỗi khi không đặt ngoặc kép', () => {
    const loose = valuesOf(spec('start'), tokenize('ideas/y tuong moi.txt'))
    expect(loose.extra).toBe('tuong')
    expect(checkLine(spec('start'), loose.values, { ...base, quoted: loose.quoted, extra: loose.extra }).err).toContain('khoảng trắng')
    const quoted = valuesOf(spec('start'), tokenize('"ideas/y tuong moi.txt"'))
    expect(checkLine(spec('start'), quoted.values, { ...base, quoted: quoted.quoted }).err).toBe('')
  })

  it('export hợp lệ → mô tả đích, khoảng và cảnh báo ghi đè', () => {
    const parsed = valuesOf(spec('export'), tokenize('exports/tra-xanh.epub to=30'))
    const result = checkLine(spec('export'), parsed.values, base)
    expect(result.err).toBe('')
    expect(result.infos[0]).toBe('Sẽ tạo ebook EPUB trong exports/tra-xanh.epub.')
    expect(result.infos[1]).toBe('Gồm chương 1–5 · bỏ qua chương 6–30 vì chưa viết xong.')
    expect(result.infos[2]).toBe('Nếu tệp đã có, lệnh sẽ dừng để giữ bản cũ.')
  })

  it('lệnh cần rảnh bị cảnh báo khi trợ lý đang viết', () => {
    const parsed = valuesOf(spec('import'), tokenize('nhap.txt'))
    expect(checkLine(spec('import'), parsed.values, { ...base, isRunning: true }).warn)
      .toBe('Trợ lý đang viết chương 6 — lệnh này sẽ bị từ chối cho tới khi hệ thống rảnh.')
  })

  it('/start chỉ chạy ở màn hình chào mừng', () => {
    const parsed = valuesOf(spec('start'), tokenize('ideas/thiet-lap.md'))
    expect(checkLine(spec('start'), parsed.values, base).warn)
      .toBe('Chỉ chạy ở màn hình chào mừng. Đang mở truyện thì dùng màn Truyện mới.')
  })
})

describe('composeLine', () => {
  it('tự đặt ngoặc kép cho tên tệp có khoảng trắng và đọc lại được', () => {
    const line = composeLine(spec('start'), { path: 'ideas/y tuong moi.txt' })
    expect(line).toBe('/start "ideas/y tuong moi.txt"')
    expect(valuesOf(spec('start'), tokenize(line).slice(1)).values.path).toBe('ideas/y tuong moi.txt')
  })

  it('công tắc và tiền tố viết liền đúng cú pháp', () => {
    const line = composeLine(spec('export'), { path: 'exports/tra-xanh.epub', from: '1', to: '30', ow: true })
    expect(line).toBe('/export exports/tra-xanh.epub from=1 to=30 --overwrite')
  })
})

describe('buildSlashView', () => {
  it('/ex — gợi ý export, ghost phần còn lại, Enter điền vì có tham số', () => {
    const view = buildSlashView('/ex', ctx())
    expect(view?.mode).toBe('list')
    expect(view?.items[0]?.key).toBe('export')
    expect(view?.ghost).toBe('port')
    expect(view?.enterHint).toBe('chạy')
    expect(acceptSlash(view!, 0)).toEqual({ next: '/export ' })
  })

  it('/help — Enter chạy thẳng vì không có tham số', () => {
    const view = buildSlashView('/help', ctx())
    expect(view?.items[0]?.key).toBe('help')
    expect(acceptSlash(view!, 0)).toEqual({ submit: '/help' })
  })

  it('index mục trong danh sách phải liên tục và duy nhất — mỗi phím ↓ chỉ chọn đúng 1 mục', () => {
    const view = buildSlashView('/', ctx())
    const indices = view!.items.map((item) => item.index)
    expect(new Set(indices).size).toBe(indices.length)
    expect(indices).toEqual(indices.map((_, i) => i))
    expect(view!.sections.flatMap((s) => s.items).map((item) => item.index)).toEqual(indices)
  })

  it('/review — enum on|off, ô đang nhập là Chế độ', () => {
    const view = buildSlashView('/review ', ctx())
    expect(view?.mode).toBe('args')
    expect(view?.items.map((item) => item.token)).toEqual(['on', 'off'])
    expect(view?.detail.slot?.title).toBe('Đang nhập: Chế độ · bắt buộc')
    expect(view?.detail.usage.find((token) => token.slot)?.text).toBe('on|off')
  })

  it('/export from= — gợi ý số lấy từ tiến độ', () => {
    const view = buildSlashView('/export from=', ctx())
    expect(view?.items.map((item) => item.token)).toEqual(['from=1', 'from=6'])
    expect(view?.items[1]?.desc).toBe('Tiếp sau lần xuất trước (1–5)')
  })

  it('/exprt — tên lạ gợi ý đúng lệnh, báo lỗi và Enter sửa tên', () => {
    const view = buildSlashView('/exprt ', ctx())
    expect(view?.mode).toBe('unknown')
    expect(view?.items[0]?.key).toBe('export')
    expect(view?.hasError).toBe(true)
    expect(view?.enterHint).toBe('sửa thành /export')
    expect(acceptSlash(view!, 0)).toEqual({ next: '/export ' })
  })

  it('busy + lệnh cần rảnh → không cho chạy, có cảnh báo', () => {
    const view = buildSlashView('/import nhap.txt', ctx({ isRunning: true }))
    expect(view?.detail.status?.tone).toBe('warn')
    expect(view?.submitLine).toBeNull()
    expect(view?.detail.busyWarn).toBe(true)
    expect(acceptSlash(view!, 0)).toEqual({})
  })

  it.each(['start', 'importsim'])('/%s thiếu tệp bắt buộc → không báo “Đã đủ thông tin · Enter để chạy”', (name) => {
    const view = buildSlashView(`/${name} `, ctx())
    expect(view?.submitLine).toBeNull()
    expect(view?.enterHint).toBe('chưa chạy được')
    expect(view?.empty?.title).not.toBe('Đã đủ thông tin')
    expect(view?.empty?.title).toMatch(/^Đang gõ /)
    expect(view?.empty?.text).toBe(view?.detail.status?.text)
    expect(view?.empty?.text).toMatch(/^Còn thiếu /)
  })

  it('lệnh cần rảnh không còn tuỳ chọn khi đang viết → khung trái giữ nguyên chữ artboard, khung phải báo bị từ chối', () => {
    const view = buildSlashView('/importsim hoso.json', ctx({ isRunning: true }))
    expect(view?.submitLine).toBeNull()
    expect(view?.empty?.text).toBe('Không còn tuỳ chọn nào để thêm. Nhấn Enter để chạy.')
    expect(view?.detail.status?.text).toBe('Sẽ bị từ chối: hệ thống đang viết.')
  })

  it('đủ tham số bắt buộc → vẫn báo “Đã đủ thông tin”', () => {
    const view = buildSlashView('/importsim hoso.json ', ctx())
    expect(view?.submitLine).toBe('/importsim hoso.json')
    expect(view?.empty).toEqual({ title: 'Đã đủ thông tin', text: 'Không còn tuỳ chọn nào để thêm. Nhấn Enter để chạy.' })
  })

  it('không bắt đầu bằng / → không có popover', () => {
    expect(buildSlashView('xin chào', ctx())).toBeNull()
  })
})

describe('lệnh dùng gần đây', () => {
  beforeEach(() => {
    const store = new Map<string, string>()
    Object.defineProperty(globalThis, 'localStorage', {
      configurable: true,
      value: {
        getItem: (key: string) => store.get(key) ?? null,
        setItem: (key: string, value: string) => { store.set(key, value) },
      },
    })
  })

  it('giữ 3 lệnh gần nhất, không trùng', () => {
    pushRecentCommand('export')
    pushRecentCommand('import')
    pushRecentCommand('export')
    pushRecentCommand('sync')
    pushRecentCommand('review')
    expect(readRecentCommands()).toEqual(['review', 'sync', 'export'])
  })
})
