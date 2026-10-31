import { describe, expect, it } from 'vitest'
import {
  OTHER_TOOL, activeToolDoing, activeToolLabel, agentLabel, agentProgressLine, agentRoleLabel, cacheEventLabel,
  chapterRangeLabel, contextMemoryNote, contractStatusLabel, decisionLabel, errorLabel, fieldLabel,
  fieldValueLabel, foreshadowActionLabel, formatToolLabel, humanizeToolText, modelSwitchedNote,
  parseToolSummary, severityLabel, toolCallText, toolChipText, toolEventText, toolIcon, toolLabel,
  verdictLabel, warningLabel,
} from './labels'
import type { ActivityEvent } from '../api/types'
import { translateHost } from '../i18n/hostStrings'
import { eventView } from '../screens/workspace/events'
import realActivity from './real-activity.fixture.json'
import { activityText } from './activity'

import { statusText } from '../screens/workspace/RunHeader'
import { toolCode } from '../screens/workspace/events'

describe('lỗi hợp đồng và vai trò không lộ mã nội bộ', () => {
  const diagnostic = 'đầu ra vẫn lỗi sau 3 lần tự sửa: Không tìm thấy JSON object trong đầu ra'
  const cases = [
    ['arbiter_intervention', 'Trọng tài'],
    ['arbiter_plan_start', 'Trọng tài'],
    ['arbiter_failure', 'Trọng tài'],
    ['architect_ledger', 'Kiến trúc sư'],
    ['import_segment', 'Nhập truyện'],
    ['import_chapter_analysis', 'Nhập truyện'],
    ['import_range_digest', 'Nhập truyện'],
    ['import_book_synthesis', 'Nhập truyện'],
    ['simulation_source_report', 'Mô phỏng văn phong'],
    ['simulation_synthesis', 'Mô phỏng văn phong'],
    ['chapter_revision_analysis', 'Phân tích bản sửa'],
    ['userrules_normalize', 'Chuẩn hoá luật viết'],
    ['future_contract', 'Trợ lý'],
  ] as const

  it.each(cases)('%s được dịch ở nhật ký, trạng thái và bảng lỗi', (id, assistant) => {
    const raw = `Phán định can thiệp thất bại: arbiter: ${id}: ${diagnostic} (chưa thay đổi gì)`
    const event = { id: 'contract-error', time: '2026-10-31T09:00:00Z', summary: raw,
      category: 'ERROR', agent: 'arbiter', level: 'error', failed: true, result: raw } as ActivityEvent
    const view = eventView(event, Date.parse(event.time))
    for (const visible of [errorLabel(raw), humanizeToolText(raw), translateHost(raw), activityText(event), view.text, view.title, statusText(raw, 'error')]) {
      expect(visible).toContain(assistant)
      expect(visible).not.toMatch(/\b[A-Za-z][A-Za-z0-9]*_[A-Za-z0-9_]+\b|\b(?:arbiter|architect|writer|editor)\b|JSON object/i)
    }
    expect(toolCode(event)).toContain(raw)
    expect(event.summary).toBe(raw)
  })

  it.each(['arbiter', 'architect_long', 'architect_short', 'writer', 'editor'])('dịch tiền tố vai %s cả khi không có mã hợp đồng', (role) => {
    const raw = `${role}: unknown provider error: key=value`
    expect(errorLabel(raw)).toContain(agentLabel(role))
    expect(humanizeToolText(raw)).not.toMatch(/\b[A-Za-z][A-Za-z0-9]*_[A-Za-z0-9_]+\b|unknown|provider|key=value/i)
  })
})
describe('toolLabel — bảng §5', () => {
  it('công cụ có trong bảng → nhãn thường', () => {
    expect(toolLabel('commit_chapter')).toBe('Lưu chương')
    expect(toolLabel('draft_chapter')).toBe('Viết bản nháp')
    expect(toolLabel('novel_context')).toBe('Đọc bối cảnh')
    expect(toolLabel('audit_foundation')).toBe('Soát nền truyện')
  })

  it('công cụ lạ → tên engine chữ thường có cách, rỗng → “Công cụ khác”', () => {
    expect(toolLabel('export_book')).toBe('export book')
    expect(toolLabel('')).toBe(OTHER_TOOL)
  })

  it('icon theo loại công cụ, lạ → code', () => {
    expect(toolIcon('novel_context')).toBe('note-text')
    expect(toolIcon('read_chapter')).toBe('book-1')
    expect(toolIcon('edit_chapter')).toBe('magicpen')
    expect(toolIcon('check_consistency')).toBe('shield-tick')
    expect(toolIcon('commit_chapter')).toBe('folder-open')
    expect(toolIcon('plan_chapter')).toBe('routing-2')
    expect(toolIcon('score_chapter')).toBe('chart-2')
    expect(toolIcon('dung_do_la')).toBe('code')
  })

  it('save_foundation theo loại: premise / outline', () => {
    expect(toolLabel('save_foundation', 'premise')).toBe('Lưu tiền đề')
    expect(toolLabel('save_foundation', 'outline')).toBe('Lưu dàn ý')
    expect(toolLabel('save_foundation', 'layered_outline')).toBe('Lưu dàn ý')
    expect(toolLabel('save_foundation')).toBe('Lưu nền truyện')
    expect(toolLabel('save_foundation', 'loại-lạ')).toBe('Lưu nền truyện')
  })
})

describe('mẫu `{nhãn} · chương {n}`', () => {
  it('chữ thường “chương”, bỏ số chương khi không có', () => {
    expect(formatToolLabel('Lưu chương', 4)).toBe('Lưu chương · chương 4')
    expect(formatToolLabel('Lưu chương', null)).toBe('Lưu chương')
    expect(formatToolLabel('Lưu chương', 0)).toBe('Lưu chương')
  })
})

// Bug 2710: model tự truyền `chapter: 2147483647` (canh math.MaxInt32 của Go) hoặc một số chương
// ngoài phạm vi truyện; nhật ký từng hiện nguyên số đó (“Đọc bối cảnh · chương 2147483647”).
describe('số chương vô lý không bao giờ ra giao diện', () => {
  const sentinel = 2147483647

  it('canh MaxInt32 bị bỏ khỏi mọi mẫu nhãn', () => {
    expect(formatToolLabel('Đọc bối cảnh', sentinel)).toBe('Đọc bối cảnh')
    expect(humanizeToolText(`novel_context(chương ${sentinel})`)).toBe('Đọc bối cảnh')
    expect(agentProgressLine('writer', 'novel_context', sentinel)).toBe('Người viết · đang đọc bối cảnh')
  })

  it('số chương lớn hơn tổng số chương của truyện bị bỏ', () => {
    const event = { summary: `novel_context(chương 100)`, agent: 'architect' }
    expect(toolEventText(event, 64)).toBe('Đọc bối cảnh')
    expect(toolCallText({ tool: 'novel_context', chapter: 100, variant: '', extras: [], error: '' }, 64)).toBe('Đọc bối cảnh')
    // Không truyền tổng (chưa tải xong snapshot) → chỉ chặn canh, giữ số chương thật.
    expect(toolEventText(event)).toBe('Đọc bối cảnh · chương 100')
    // Trong phạm vi → giữ nguyên.
    expect(toolEventText({ summary: 'novel_context(chương 63)' }, 64)).toBe('Đọc bối cảnh · chương 63')
  })

  it('dòng lỗi cũng không lộ số chương canh', () => {
    expect(toolEventText({ summary: `commit_chapter(chương ${sentinel}) lỗi: hết giờ` })).toBe('Lưu chương không thành công: hết giờ')
    expect(eventView({ id: 'x', time: '2026-10-26T10:00:00Z', summary: `plan_chapter(chương ${sentinel})`, agent: 'architect', depth: 1 } as never, 0).title).toBe('Lập kế hoạch chương')
  })
})

describe('chuỗi host của FixViBackend (displayToolName)', () => {
  it('tool(chương N)', () => {
    expect(humanizeToolText('commit_chapter(chương 8)')).toBe('Lưu chương · chương 8')
    expect(humanizeToolText('plan_chapter(chương 1)')).toBe('Lập kế hoạch chương · chương 1')
  })

  it('read_chapter: bản nháp / thoại', () => {
    expect(humanizeToolText('read_chapter(chương 1 · bản nháp)')).toBe('Đọc chương · chương 1 · bản nháp')
    expect(humanizeToolText('read_chapter(chương 2 · thoại Tri Hạ)')).toBe('Đọc chương · chương 2 · thoại Tri Hạ')
  })

  it('save_foundation[type]', () => {
    expect(humanizeToolText('save_foundation[premise]')).toBe('Lưu tiền đề')
    expect(humanizeToolText('save_foundation[outline]')).toBe('Lưu dàn ý')
  })

  it('save_review: phạm vi + phán quyết', () => {
    expect(humanizeToolText('save_review(toàn cục·polish)')).toBe('Lưu thẩm duyệt · toàn truyện · gọt giũa')
    expect(humanizeToolText('save_review(chương 3·accept)')).toBe('Lưu thẩm duyệt · chương 3 · đạt')
    expect(humanizeToolText('save_review(cung này)')).toBe('Lưu thẩm duyệt · hồi này')
  })

  it('lỗi giữ thông báo, bỏ tên gốc', () => {
    expect(humanizeToolText('commit_chapter(chương 5) lỗi: kết nối bị ngắt')).toBe('Lưu chương 5 không thành công: kết nối bị ngắt')
    expect(humanizeToolText('read_chapter(chương 5) lỗi: hết giờ')).toBe('Đọc chương 5 không thành công: hết giờ')
    expect(humanizeToolText('novel_context(chương 5) lỗi: hết giờ')).toBe('Đọc bối cảnh · chương 5 không thành công: hết giờ')
  })

  it('công cụ lạ → tên engine liền chữ thường, không lỗi', () => {
    expect(humanizeToolText('export_book(chương 2)')).toBe('export book · chương 2')
  })

  it('dạng tiếng Trung cũ vẫn nhận ra số chương', () => {
    expect(humanizeToolText('read_chapter(第1章)')).toBe('Đọc chương · chương 1')
  })

  it('chuỗi không phải lời gọi công cụ → nguyên văn', () => {
    expect(humanizeToolText('Viết lại chương 4')).toBe('Viết lại chương 4')
    expect(humanizeToolText('Thử lại (2/7): lỗi mạng')).toBe('Thử lại (2/7): lỗi mạng')
    expect(parseToolSummary('Lưu chương 5')).toBeNull()
  })

  it('“writer → commit_chapter(chương 4)” → “Người viết · đang lưu chương 4”', () => {
    expect(humanizeToolText('writer → commit_chapter(chương 4)')).toBe('Người viết · đang lưu chương 4')
    expect(humanizeToolText('architect → save_foundation[premise]')).toBe('Kiến trúc sư · đang lưu tiền đề')
  })
})

describe('sự kiện công cụ', () => {
  it('sự kiện thật: câu chọn người lập kế hoạch và giao việc không lộ tên vai hay công cụ', () => {
    const expected = [
      'Kiến trúc sư · Lập nền tảng và đại cương cho ...',
      'Bắt đầu sáng tác (người lập kế hoạch: Kiến trúc sư — Người dùng yêu cầu rõ truyện ngắn đơn tập, đúng 8 chương, khoảng 12.000 từ và kết trọn vẹn trong một tập; đáp ứng điều kiện chọn Kiến trúc sư với dung lượng không quá 25 chương.)',
      'Biên tập viên · Thẩm duyệt 5 chương đầu: gọi Đọc bối cảnh',
    ]
    // Gói O mục 3: dòng hiển thị kẹp “lý do” ≤80 ký tự; toàn văn còn trong view.title (tooltip).
    const clamped = [
      null,
      'Bắt đầu sáng tác (người lập kế hoạch: Kiến trúc sư — Người dùng yêu cầu rõ truyện ngắn đơn tập, đúng 8 chương, khoảng 12.000 từ và kế…)',
      null,
    ]
    for (const [index, fixture] of realActivity.entries()) {
      const event = fixture as ActivityEvent
      const desktop = eventView(event, Date.parse(event.time))
      const sentence = index === 2 ? 'Biên tập viên · Thẩm duyệt 5 chương đầu' : expected[index]
      expect(`${desktop.who}${desktop.text}`).toBe(clamped[index] ?? sentence)
      const phone = activityText(event)
      expect(desktop.title).toBe(phone)
      expect(humanizeToolText(event.summary)).toBe(expected[index])
      expect(translateHost(event.summary)).toBe(expected[index])
      expect(`${desktop.who}${desktop.text}`).not.toMatch(/\b(?:architect_short|architect_long|architect|writer|editor|arbiter|novel_context)\b/)
      expect(phone).toBe(event.agent ? sentence.split(' · ')[1] : sentence)
    }
    for (const role of ['architect_short', 'architect_long', 'architect', 'writer', 'editor', 'arbiter']) {
      expect(humanizeToolText(`Chuyển cho ${role} để gọi novel_context(chapter=7)`)).toBe(`Chuyển cho ${agentLabel(role)} để gọi ${toolLabel('novel_context')}`)
    }
  })

  it('lấy số chương/loại từ args khi summary thiếu', () => {
    expect(toolEventText({ summary: 'commit_chapter', args: '{"chapter":4}' })).toBe('Lưu chương · chương 4')
    expect(toolEventText({ summary: 'save_foundation', args: '{"type":"outline"}' })).toBe('Lưu dàn ý')
    expect(toolEventText({ summary: 'save_foundation', chapter: 7 })).toBe('Lưu nền truyện')
  })

  it('bản giao việc thật của host không lộ tên công cụ hay khoá type=/scope=', () => {
    const cases: Array<[string, string]> = [
      ['architect_long (Mở rộng cung 2 của tập 1 (save_foundation type=expand_arc))', 'Kiến trúc sư · Mở rộng cung 2 của tập 1'],
      ['editor (Tạo tóm tắt tập 1 (save_volume_summary))', 'Biên tập viên · Tạo tóm tắt tập 1'],
      ['editor (Thẩm duyệt toàn cục 8 chương đầu (save_review scope=global, chapter=8))', 'Biên tập viên · Thẩm duyệt toàn cục 8 chương đầu'],
    ]
    for (const [raw, expected] of cases) {
      expect(humanizeToolText(raw)).toBe(expected)
      expect(humanizeToolText(raw)).not.toMatch(/[a-z]+_[a-z_]+|\b(?:type|scope|chapter)=/)
    }
    const engine = humanizeToolText('Gọi save_foundation(type=expand_arc) để mở rộng cung kế tiếp; nếu tập hiện tại đã viết xong, dùng type=append_volume để thêm tập kế tiếp.')
    expect(engine).toBe('Gọi “Lưu dàn ý” để mở rộng cung kế tiếp; nếu tập hiện tại đã viết xong, dùng “Lưu dàn ý” để thêm tập kế tiếp.')
  })

  it('summary đã là lời thường → null (giữ nguyên câu của host)', () => {
    expect(toolEventText({ summary: 'Đã đọc bản nháp chương 5' })).toBeNull()
  })

  it('chip điện thoại luôn là nhãn thường', () => {
    expect(toolChipText({ tool: 'draft_chapter' })).toBe('Viết bản nháp')
    expect(toolChipText({ tool: 'save_foundation', args: '{"type":"premise"}' })).toBe('Lưu tiền đề')
    expect(toolChipText({ tool: 'mystery_tool' })).toBe('mystery tool')
  })

  it('toolCallText ghép đủ mảnh', () => {
    expect(toolCallText({ tool: 'read_chapter', chapter: 3, variant: '', extras: ['bản nháp'], error: '' })).toBe('Đọc chương · chương 3 · bản nháp')
  })
})

describe('vai trò & dòng tiến độ', () => {
  it('nhãn vai', () => {
    expect(agentRoleLabel('writer')).toBe('Người viết')
    expect(agentRoleLabel('ARCHITECT_LONG')).toBe('Kiến trúc sư')
    expect(agentRoleLabel('host')).toBe('')
  })

  it('agentLabel: system/agent và vai lạ → chữ thường', () => {
    expect(agentLabel('writer')).toBe('Người viết')
    expect(agentLabel('system')).toBe('Hệ thống')
    expect(agentLabel('agent')).toBe('trợ lý')
    expect(agentLabel('custom_agent')).toBe('custom agent')
    expect(agentLabel('')).toBe('')
  })

  it('agentProgressLine', () => {
    expect(agentProgressLine('writer', 'commit_chapter', 4)).toBe('Người viết · đang lưu chương 4')
    expect(agentProgressLine('writer', 'draft_chapter', 4)).toBe('Người viết · đang viết bản nháp chương 4')
    expect(agentProgressLine('writer', 'commit_chapter')).toBe('Người viết · đang lưu chương')
    expect(agentProgressLine('editor', '')).toBe('Biên tập viên')
    expect(agentProgressLine('writer', 'export_book', 2)).toBe('Người viết · đang export book · chương 2')
  })

  it('activeToolLabel viết hoa chữ đầu', () => {
    expect(activeToolLabel('commit_chapter')).toBe('Đang lưu chương')
    expect(activeToolLabel('save_foundation', 'premise')).toBe('Đang lưu tiền đề')
  })

  it('activeToolDoing viết thường, có kiểu và cụm dự phòng', () => {
    expect(activeToolDoing('save_foundation', 'outline')).toBe('đang lưu dàn ý')
    expect(activeToolDoing('save_foundation', 'premise')).toBe('đang lưu tiền đề')
    expect(activeToolDoing('')).toBe('đang làm việc')
  })
})

describe('trường khối lưu chương', () => {
  it('nhãn trường; trường kỹ thuật bị ẩn', () => {
    expect(fieldLabel('hook_type')).toBe('Kiểu móc')
    expect(fieldLabel('dominant_strand')).toBe('Tuyến chính')
    expect(fieldLabel('feedback')).toBe('Góp ý')
    expect(fieldLabel('cast_intros')).toBe('Nhân vật mới')
    expect(fieldLabel('characters')).toBe('Nhân vật xuất hiện')
    expect(fieldLabel('foreshadow_updates')).toBe('Chi tiết cài cắm')
    expect(fieldLabel('key_events')).toBe('Sự kiện chính')
    for (const hidden of ['replace_all', 'max_runes', 'id']) expect(fieldLabel(hidden)).toBeNull()
  })

  it('dịch giá trị', () => {
    expect(fieldValueLabel('hook_type', 'crisis')).toBe('khủng hoảng')
    expect(fieldValueLabel('hook_type', 'cliffhanger')).toBe('treo')
    expect(fieldValueLabel('dominant_strand', 'quest')).toBe('nhiệm vụ')
    expect(fieldValueLabel('dominant_strand', 'romance')).toBe('tình cảm')
    expect(fieldValueLabel('hook_type', 'twist')).toBe('Khác')
    expect(fieldValueLabel('hook_type', 'null')).toBeNull()
    expect(fieldValueLabel('feedback', null)).toBeNull()
  })

  it('tham số công cụ theo bảng 07-copy', () => {
    expect(fieldLabel('target_words')).toBe('Độ dài mục tiêu')
    expect(fieldLabel('words')).toBe('Số chữ')
    expect(fieldLabel('arc')).toBe('Hồi')
    expect(fieldLabel('source')).toBe('Bản')
    expect(fieldLabel('foreshadow')).toBe('Chi tiết cài cắm')
    expect(fieldLabel('from')).toBe('Chương')
    expect(fieldLabel('to')).toBe('Chương')
    expect(chapterRangeLabel(2, 5)).toBe('Chương 2–5')
  })

  it('giá trị tham số: bản nháp, số chữ', () => {
    expect(fieldValueLabel('source', 'draft')).toBe('nháp')
    expect(fieldValueLabel('target_words', '2000')).toBe('2.000 chữ')
    expect(fieldValueLabel('words', '58699')).toBe('58.699')
  })

  it('foreshadow action', () => {
    expect(foreshadowActionLabel('plant')).toBe('Đã gieo')
    expect(foreshadowActionLabel('advance')).toBe('Đẩy tiếp')
    expect(foreshadowActionLabel('resolve')).toBe('Thu hồi')
  })
})

describe('nhãn thẩm duyệt (Trình đọc)', () => {
  it('verdict / severity / contract', () => {
    expect(verdictLabel('accept')).toBe('Đạt')
    expect(verdictLabel('polish')).toBe('Cần gọt giũa')
    expect(verdictLabel('rewrite')).toBe('Viết lại')
    expect(severityLabel('critical')).toBe('nghiêm trọng')
    expect(severityLabel('error')).toBe('lỗi')
    expect(severityLabel('warning')).toBe('lưu ý')
    expect(contractStatusLabel('met')).toBe('đạt')
    expect(contractStatusLabel('partial')).toBe('một phần')
    expect(contractStatusLabel('missed')).toBe('chưa đạt')
    expect(verdictLabel('???')).toBe('Khác')
  })
})

describe('phán quyết, cảnh báo, lỗi — 07-copy', () => {
  it('retry · reroute · abort', () => {
    expect(decisionLabel('retry')).toBe('thử lại')
    expect(decisionLabel('reroute')).toBe('giao trợ lý khác')
    expect(decisionLabel('abort')).toBe('dừng hẳn')
    expect(decisionLabel('escalate_now')).toBe('escalate now')
  })

  it('cảnh báo & bộ nhớ đệm', () => {
    expect(warningLabel('StopGuard')).toBe('Bộ chống lặp')
    expect(warningLabel('stop_guard')).toBe('Bộ chống lặp')
    expect(cacheEventLabel('read')).toBe('Đọc bộ nhớ đệm')
    expect(cacheEventLabel('write')).toBe('Ghi bộ nhớ đệm')
    expect(cacheEventLabel('break')).toBe('Lần mất bộ nhớ đệm')
    expect(contextMemoryNote('writer', 92, '184.000', '200.000'))
      .toBe('Người viết đã dùng 92% bộ nhớ ngữ cảnh (184.000/200.000) · sẽ tóm tắt phần cũ để viết tiếp')
    expect(modelSwitchedNote('writer', 'openai/gpt-6.1-sol')).toBe('Đã đổi model cho Người viết → openai · gpt-6.1-sol')
  })

  it('lỗi engine → câu thường; lời thường tiếng Việt giữ nguyên', () => {
    expect(errorLabel('429 Too Many Requests', 'OpenAI')).toBe('OpenAI báo vượt hạn mức (mã 429)')
    expect(errorLabel('HTTP 429')).toBe('Nhà cung cấp báo vượt hạn mức (mã 429)')
    expect(errorLabel('ENOSPC')).toBe('Ổ đĩa máy chủ đã đầy')
    expect(errorLabel('write failed: ENOSPC: no space left on device')).toBe('Ổ đĩa máy chủ đã đầy')
    expect(errorLabel('lỗi lạ quá')).toBe('lỗi lạ quá')
    expect(errorLabel(null)).toBe('')
  })
})
