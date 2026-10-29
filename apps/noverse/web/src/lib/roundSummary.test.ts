import { describe, expect, it } from 'vitest'
import { parseNodes, parseStream } from './streamRounds'
import { matchRoundEvents, nodesFromArgs, roundAgentOf, toolResultLine, toolSummary, type ToolEvent } from './roundSummary'
import type { StreamRound } from './streamRounds'

// Bảng tóm tắt của MỌI công cụ writer/architect/editor/arbiter gọi (internal/agents/build.go).
// `args` là đúng định dạng host phát ra (internal/host/stream_extract.go), không phải JSON.
const TABLE: { tool: string; args: string; summary: string }[] = [
  // ── Đọc & soát ──
  { tool: 'novel_context', args: 'chapter: 6', summary: 'Xem bối cảnh chương 6' },
  { tool: 'novel_context', args: '', summary: 'Xem bối cảnh truyện' },
  { tool: 'read_chapter', args: 'chapter: 234\nsource: final', summary: 'Đọc lại chương 234' },
  { tool: 'read_chapter', args: 'chapter: 5\nsource: draft', summary: 'Đọc lại bản nháp chương 5' },
  { tool: 'read_chapter', args: 'from: 1\nto: 3\nsource: final', summary: 'Đọc chương 1–3' },
  { tool: 'read_chapter', args: 'from: 1\nto: 3\nsource: draft', summary: 'Đọc chương 1–3 · bản nháp' },
  { tool: 'read_chapter', args: 'from: 7\nto: 0\nsource: final', summary: 'Đọc từ chương 7' },
  { tool: 'check_consistency', args: 'chapter: 6', summary: 'Soát nhất quán chương 6' },
  { tool: 'check_consistency', args: '', summary: 'Soát nhất quán' },
  // ── Writer ──
  { tool: 'plan_chapter', args: 'chapter: 5\ntitle: Bữa cơm trưa của người vắng mặt', summary: 'Lập kế hoạch chương 5 · “Bữa cơm trưa của người vắng mặt”' },
  { tool: 'plan_chapter', args: 'chapter: 5\ntitle: null', summary: 'Lập kế hoạch chương 5' },
  { tool: 'draft_chapter', args: 'chapter: 5\nmode: write', summary: 'Viết bản nháp chương 5' },
  { tool: 'draft_chapter', args: 'chapter: 5\nmode: append', summary: 'Viết tiếp chương 5' },
  { tool: 'edit_chapter', args: 'chapter: 4\nreplace_all: false', summary: 'Gọt giũa chương 4' },
  { tool: 'edit_chapter', args: 'chapter: 4\nreplace_all: true', summary: 'Sửa cả chương 4' },
  { tool: 'commit_chapter', args: 'chapter: 4\nhook_type: lat_thong_tin', summary: 'Nộp chương 4' },
  // ── Editor ──
  { tool: 'save_review', args: 'chapter: 3\nscope: chapter\nverdict: polish', summary: 'Thẩm duyệt chương 3' },
  { tool: 'save_review', args: 'chapter: 8\nscope: global\nverdict: accept', summary: 'Thẩm duyệt toàn truyện' },
  { tool: 'save_review', args: 'chapter: 8\nscope: arc\narc: 2', summary: 'Thẩm duyệt cung 2' },
  { tool: 'save_arc_summary', args: 'volume: 1\narc: 2\ntitle: Khế ước bán thân', summary: 'Tóm tắt cung 2 · tập 1' },
  { tool: 'save_arc_summary', args: 'arc: 2', summary: 'Tóm tắt cung 2' },
  { tool: 'save_volume_summary', args: 'volume: 2\ntitle: Tập hai', summary: 'Tóm tắt tập 2' },
  // ── Architect ──
  { tool: 'save_book', args: 'title: Tuổi Sáng Phản Bộn\nsynopsis: Một nữ thư ký…', summary: 'Lưu thông tin truyện “Tuổi Sáng Phản Bộn”' },
  { tool: 'save_book', args: 'synopsis: Một nữ thư ký…', summary: 'Lưu thông tin truyện' },
  { tool: 'save_foundation', args: 'type: premise\ncontent:\n  text: Một nữ thư ký…', summary: 'Lưu tiền đề truyện' },
  { tool: 'save_foundation', args: 'type: characters\ncontent:\n  characters:\n    - name: Lâm Tri Hạ\n    - name: Tống Nhược Vy', summary: 'Lưu hồ sơ 2 nhân vật' },
  { tool: 'save_foundation', args: 'type: characters\n- name: Lâm Tri Hạ\n- name: Tống Nhược Vy', summary: 'Lưu hồ sơ 2 nhân vật' },
  { tool: 'save_foundation', args: 'type: characters', summary: 'Lưu hồ sơ nhân vật' },
  { tool: 'save_foundation', args: 'type: world_rules\ncontent:\n  - Show phát trực tiếp\n  - Kịch bản có nhật ký sửa đổi\n  - Nhược Vy độc quyền Tinh Mang', summary: 'Lưu 3 quy tắc thế giới' },
  { tool: 'save_foundation', args: 'type: outline\ncontent:\n  - chapter: 1\n    title: Chữ ký sai hợp đồng\n  - chapter: 2\n    title: Phòng quy hoạch vắng chủ', summary: 'Lưu dàn ý · 2 chương' },
  { tool: 'save_foundation', args: 'type: layered_outline\ncontent:\n  - volume: 1\n    title: Tập một\n  - volume: 2\n    title: Tập hai', summary: 'Lưu dàn ý · 2 tập' },
  { tool: 'save_foundation', args: 'type: append_volume\nvolume: 2\nreason: Còn tuyến phụ chưa kết', summary: 'Thêm tập mới' },
  { tool: 'save_foundation', args: 'type: update_compass\ncontent:\n  diem_khac: …', summary: 'Cập nhật hướng đi' },
  { tool: 'save_foundation', args: 'type: complete_book\nreason: Đã hết truyện', summary: 'Hoàn tất truyện' },
  { tool: 'save_foundation', args: '', summary: 'Lưu thiết lập' },
  { tool: 'revise_outline', args: 'from_chapter: 25\nreason: Người dùng đổi hướng', summary: 'Sửa đại cương từ chương 25' },
  { tool: 'revise_outline', args: 'from_chapter: 0', summary: 'Sửa đại cương' },
  { tool: 'resolve_outline_feedback', args: 'reason: Đã sửa theo góp ý', summary: 'Xử lý góp ý đại cương' },
  { tool: 'audit_foundation', args: 'fingerprint: abc', summary: 'Soát lại nền truyện' },
  // ── Ngoài bảng ──
  { tool: 'reopen_book', args: 'chapters:\n  - 3\n  - 4\nreason: Sửa tay', summary: 'Mở lại truyện · 2 chương' },
  { tool: 'reopen_book', args: '', summary: 'Mở lại truyện' },
  { tool: 'export_book', args: '', summary: '' },
]

describe('toolSummary — một câu cho mỗi công cụ', () => {
  for (const row of TABLE) {
    it(`${row.tool}${row.args ? ` · ${row.args.split('\n')[0]}` : ' · không tham số'} → ${row.summary || '(nhãn bước)'}`, () => {
      expect(toolSummary(row.tool, parseNodes(row.args))).toBe(row.summary)
    })
  }
})

// Ảnh chụp bug 2710: “Đọc chương · Chương 2 · Chương 0 · Chương 0” — args thật có from/to = 0.
const SCREENSHOT = 'chapter: 2\nsource: draft\nfrom: 0\nto: 0\nmax_runes: 10000\ncharacter: null'

describe('argv mặc định (0 / null / rỗng) không thành chip', () => {
  it('đúng thân round trong ảnh chụp → “Đọc lại bản nháp chương 2”, không có “Chương 0”', () => {
    const summary = toolSummary('read_chapter', parseNodes(SCREENSHOT))
    expect(summary).toBe('Đọc lại bản nháp chương 2')
    expect(summary).not.toContain('0')
    expect(summary).not.toContain('null')
  })

  it('mỗi nhãn chỉ hiện một lần trong câu', () => {
    for (const row of TABLE) {
      const summary = toolSummary(row.tool, parseNodes(row.args))
      const words = summary.split(' ')
      expect(new Set(words).size, `${row.tool}: “${summary}”`).toBe(words.length)
    }
  })
})

const event = (over: Partial<ToolEvent>): ToolEvent => ({
  id: 'ev', time: '', finishedAt: '', failed: false, category: 'TOOL', agent: 'writer', summary: '',
  kind: '', level: 'info', depth: 1, durationNs: 0, retryAt: '', sessionId: '', canRetryNow: false, ...over,
})

describe('số chương vô lý không hiện', () => {
  it('giá trị canh 2147483647 và số vượt tổng số chương bị bỏ', () => {
    expect(toolSummary('read_chapter', parseNodes('chapter: 2147483647\nsource: final'), 8)).toBe('Đọc chương')
    expect(toolSummary('read_chapter', parseNodes('chapter: 99\nsource: draft'), 8)).toBe('Đọc lại bản nháp')
    expect(toolSummary('novel_context', parseNodes('chapter: 2147483647'), 8)).toBe('Xem bối cảnh truyện')
    expect(toolSummary('revise_outline', parseNodes('from_chapter: 2147483647'), 8)).toBe('Sửa đại cương')
    expect(toolSummary('read_chapter', parseNodes('chapter: 8\nsource: final'), 8)).toBe('Đọc lại chương 8')
  })
})

describe('nodesFromArgs — tham số của dòng Nhật ký khi stream chỉ có đầu mục', () => {
  it('dựng câu tóm tắt cho công cụ không có extractor (save_book)', () => {
    expect(toolSummary('save_book', nodesFromArgs('{"title":"Tuổi Sáng Phản Bộn","synopsis":"Một nữ thư ký…"}'))).toBe('Lưu thông tin truyện “Tuổi Sáng Phản Bộn”')
    expect(toolSummary('save_book', nodesFromArgs('{"synopsis":"Một nữ thư ký…"}'))).toBe('Lưu thông tin truyện')
    expect(toolSummary('reopen_book', nodesFromArgs('{"chapters":[3,4],"reason":"Sửa tay"}'))).toBe('Mở lại truyện · 2 chương')
  })

  it('bỏ 0 / null / false / rỗng như `arg()`', () => {
    expect(nodesFromArgs('{"chapter":0,"from":null,"to":"","source":"draft","max_runes":10000}').map((node) => node.key))
      .toEqual(['source', 'max_runes'])
    expect(toolSummary('read_chapter', nodesFromArgs('{"chapter":2,"source":"draft","from":0,"to":0}'))).toBe('Đọc lại bản nháp chương 2')
  })

  it('args cắt cụt hoặc không phải JSON → rỗng (thẻ giữ nhãn bước)', () => {
    expect(nodesFromArgs('{"chapter":5,"content":"Bữa trưa…')).toEqual([])
    expect(nodesFromArgs(undefined)).toEqual([])
  })
})

describe('toolResultLine — kết quả của lần gọi, lời thường', () => {
  it('đọc chương: “đã đọc 1.546 chữ” (số chữ host trả, không phải JSON)', () => {
    expect(toolResultLine(event({ tool: 'read_chapter', status: 'ok', result: '{"chapter":5,"source":"draft","word_count":1546,"content":"Bữa trưa…' })))
      .toBe('đã đọc 1.546 chữ')
  })

  it('resultHint cứu số liệu khi `result` đã bị cắt 800 ký tự', () => {
    expect(toolResultLine(event({
      tool: 'read_chapter', status: 'ok', result: '{"chapter":5,"source":"draft","content":"Bữa trưa hôm ấy…',
      resultHint: '{"word_count":1546}',
    }))).toBe('đã đọc 1.546 chữ')
  })

  it('viết, nộp, soát, sửa đọc cùng trường số chữ nhưng nói đúng việc', () => {
    expect(toolResultLine(event({ tool: 'draft_chapter', status: 'ok', result: '{"written":true,"chapter":5,"word_count":3200,"next_step":"…"' })))
      .toBe('đã viết 3.200 chữ')
    expect(toolResultLine(event({ tool: 'check_consistency', status: 'ok', result: '{"chapter":6,"word_count":1546,"content":"…"' })))
      .toBe('đã soát 1.546 chữ')
    expect(toolResultLine(event({ tool: 'commit_chapter', status: 'ok', result: '{"chapter":1,"committed":true,"word_count":1546,"hook_type":"crisis"' })))
      .toBe('đã lưu · 1.546 chữ')
    expect(toolResultLine(event({ tool: 'edit_chapter', status: 'ok', result: '{"chapter":2,"word_count":1440,"diff":"…"' })))
      .toBe('đã sửa · 1.440 chữ')
  })

  it('lưu, xong, lập kế hoạch, bỏ qua', () => {
    expect(toolResultLine(event({ tool: 'save_review', status: 'ok', result: '{"saved":true,"chapter":3,"issues":2}' }))).toBe('đã lưu · 2 vấn đề')
    expect(toolResultLine(event({ tool: 'save_foundation', status: 'ok', result: '{"saved":true,"type":"premise"}' }))).toBe('đã lưu')
    expect(toolResultLine(event({ tool: 'commit_chapter', status: 'ok', result: '{"ok":true}' }))).toBe('xong')
    expect(toolResultLine(event({ tool: 'plan_chapter', status: 'ok', result: '{"planned":true,"chapter":5}' }))).toBe('đã lập kế hoạch')
    expect(toolResultLine(event({ tool: 'draft_chapter', status: 'ok', result: '{"skipped":true,"completed":true}' }))).toBe('bỏ qua vì chương đã nộp')
  })

  it('thiết lập: số nhân vật / quy tắc đã lưu, nền truyện soát xong', () => {
    expect(toolResultLine(event({ tool: 'save_foundation', args: '{"type":"characters"}', status: 'ok', result: '{"count":7,"saved":true,"type":"characters"}' })))
      .toBe('đã lưu 7 nhân vật')
    expect(toolResultLine(event({ tool: 'save_foundation', args: '{"type":"world_rules"}', status: 'ok', result: '{"count":11,"saved":true,"type":"world_rules"}' })))
      .toBe('đã lưu 11 quy tắc')
    expect(toolResultLine(event({ tool: 'save_foundation', args: '{"type":"layered_outline"}', status: 'ok', result: '{"count":8,"saved":true,"type":"layered_outline"}' })))
      .toBe('đã lưu 8 mục')
    expect(toolResultLine(event({ tool: 'audit_foundation', status: 'ok', result: '{"foundation_ready":true,"issues":[]}' })))
      .toBe('nền truyện đã sẵn sàng')
  })

  it('lỗi thật trong nhật ký live → câu tiếng Việt, không phải chuỗi gốc', () => {
    expect(toolResultLine(event({ tool: 'read_chapter', status: 'error', failed: true, level: 'error', result: 'InputValidationError: read_chapter failed due to the following issue:\nThe required parameter `source` is missing.' })))
      .toBe('thiếu tham số bắt buộc')
    expect(toolResultLine(event({ tool: 'novel_context', status: 'error', failed: true, level: 'error', result: 'context payload exceeds budget after trimming: size=73416 budget=61440' })))
      .toBe('ngữ cảnh vượt hạn mức, đã cắt bớt')
    expect(toolResultLine(event({ tool: 'read_chapter', status: 'error', failed: true, level: 'error', result: 'no content found for chapter 3' })))
      .toBe('chưa có nội dung chương 3')
  })

  it('chưa xong / không có kết quả', () => {
    expect(toolResultLine(event({ tool: 'read_chapter', status: 'running', result: '' }))).toBe('')
    expect(toolResultLine(event({ tool: 'read_chapter', status: 'ok', result: '' }))).toBe('')
  })
})

const round = (id: string, body: string): StreamRound => {
  const parsed = parseStream(`✻ Đọc chương\n${body}`, false)
  return { ...parsed.rounds[0], id }
}

describe('matchRoundEvents — vòng ✻ ↔ dòng Nhật ký cùng lần gọi', () => {
  const rounds = [
    round('a', 'chapter: 5\nsource: draft\nfrom: 0\nto: 0'),
    round('b', 'chapter: 6\nsource: final\nfrom: 0\nto: 0'),
  ]

  it('hai lần đọc khác chương nhận đúng dòng của mình (kể cả dòng đang chạy rồi xong)', () => {
    const matched = matchRoundEvents(rounds, [
      event({ id: 'e1', tool: 'read_chapter', args: '{"chapter":5,"source":"draft"}', status: 'running', result: '' }),
      event({ id: 'e1', tool: 'read_chapter', args: '{"chapter":5,"source":"draft"}', status: 'ok', result: '{"chapter":5,"word_count":1546}' }),
      event({ id: 'e2', tool: 'read_chapter', args: '{"chapter":6,"source":"final"}', status: 'ok', result: '{"chapter":6,"word_count":1700}' }),
    ])
    expect(matched.get('a')?.result).toBe('{"chapter":5,"word_count":1546}')
    expect(matched.get('b')?.result).toBe('{"chapter":6,"word_count":1700}')
    expect(matched.size).toBe(2)
  })

  it('thiếu dòng Nhật ký → không ghép (thẻ chỉ hiện tóm tắt)', () => {
    expect(matchRoundEvents([rounds[0]], []).size).toBe(0)
  })

  it('dòng của công cụ khác không bị ghép nhầm', () => {
    const matched = matchRoundEvents([rounds[1]], [event({ id: 'x', tool: 'novel_context', result: '{"ok":true}' })])
    expect(matched.size).toBe(0)
  })
})

describe('roundAgentOf — trợ lý sở hữu công cụ', () => {
  it('công cụ riêng của Kiến trúc sư / Biên tập viên; công cụ dùng chung → rỗng', () => {
    expect(roundAgentOf('save_foundation')).toBe('architect')
    expect(roundAgentOf('save_review')).toBe('editor')
    expect(roundAgentOf('read_chapter')).toBe('')
    expect(roundAgentOf('novel_context')).toBe('')
  })
})
