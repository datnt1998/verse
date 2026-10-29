import { describe, expect, it } from 'vitest'
import type { ActivityEvent } from '../../../api/types'
import { groupChapters, initialLogFilter, logTime, nextUnseenEvents, toolArgRows, toolDetailText, toolResultText } from './logic'
import { mergeLogEvents } from '../../../api/useActivityHistory'
import { errorLabel, humanizeToolText, toolChipText, toolEventText, toolIcon } from '../../../lib/labels'
import { translateHost } from '../../../i18n/hostStrings'
import realErrors from './real-errors.fixture.json'

const event = (id: string, chapter: number, second: number, patch: Partial<ActivityEvent> = {}): ActivityEvent => ({
  id, chapter, time: `2026-10-25T14:31:${String(second).padStart(2, '0')}Z`, finishedAt: '', failed: false,
  category: 'agent', agent: 'writer', summary: `Sự kiện ${id}`, kind: 'tool', level: 'info', depth: 0,
  durationNs: 0, retryAt: '', sessionId: 'session-1', canRetryNow: false, ...patch,
})

describe('nhật ký điện thoại M6', () => {
  it('lỗi thật chỉ hiện phần tiếng Việt, giữ lỗi gốc trong nội dung chép kể cả khi API không trả result', () => {
    const raw = 'foreshadow_updates[2] references unknown id "E8": tool precondition failed'
    const failed = realErrors[0] as ActivityEvent
    const visible = toolEventText(failed)
    expect(visible).toBe('Lưu chương 7 không thành công')
    expect(visible).not.toMatch(/[a-z]+_[a-z_]+|references|unknown|tool|precondition|failed/)
    expect(humanizeToolText(failed.summary)).toBe(visible)
    expect(translateHost(failed.summary)).toBe(visible)
    expect(toolChipText({ tool: failed.tool! })).toBe('Lưu chương')
    expect(toolDetailText(failed)).toContain(raw)
    expect(toolResultText({ ...failed, result: raw })).toBe('Công cụ báo lỗi')
    expect(toolEventText(realErrors[1] as ActivityEvent)).toBe('Lưu chương 4 không thành công: chính văn và tiêu đề chương 4 không thay đổi, không phát hiện chỉnh sửa viết lại')
    for (const cause of ['upstream connection failed', 'lỗi ghi foreshadow_updates[2]', 'không ghi được /tmp/chapter.md', 'không đọc được chapter.json']) {
      expect(toolEventText({ summary: `commit_chapter(chương 7) lỗi: ${cause}` })).toBe('Lưu chương 7 không thành công')
      expect(errorLabel(cause)).toBe('')
    }
    expect(toolEventText({ summary: 'commit_chapter(chương 7) lỗi: 429 Too Many Requests' })).toBe('Lưu chương 7 không thành công: Nhà cung cấp báo vượt hạn mức (mã 429)')
    expect(toolEventText({ summary: 'commit_chapter(chương 7) lỗi: ENOSPC: no space left on device' })).toBe('Lưu chương 7 không thành công: Ổ đĩa máy chủ đã đầy')
  })

  it('nhóm theo chương giảm dần, giữ hoạt động chung và metadata, chỉ chương hiện tại có orb thở', () => {
    const groups = groupChapters([event('c24', 24, 5), event('c25a', 25, 10), event('general', 0, 1), event('c25b', 25, 20)], [
      { chapter: 24, title: 'Đèn tắt', score: 8.1, eventCount: 6 },
      { chapter: 25, title: 'Bản gốc trở về', eventCount: 4 },
    ], 25, true)
    expect(groups.map((group) => [group.chapter, group.live, group.events.map((row) => row.id)])).toEqual([
      [25, true, ['c25b', 'c25a']], [24, false, ['c24']], [0, false, ['general']],
    ])
    expect(groups[1]).toMatchObject({ title: 'Đèn tắt', score: 8.1, count: 6 })
    expect(groupChapters([event('done', 25, 30)], [], 25, false)[0].live).toBe(false)
  })

  it('gộp lịch sử phân trang với luồng trực tiếp không trùng ID và giữ bản cập nhật công cụ mới nhất', () => {
    const older = [event('old', 20, 1), event('tool', 24, 2, { tool: 'save_chapter', status: 'running' })]
    const head = [event('tool', 24, 2, { tool: 'save_chapter', status: 'running' }), event('head', 25, 5)]
    const live = [event('tool', 24, 2, { tool: 'save_chapter', status: 'ok', result: 'Đã lưu' }), event('new', 25, 10), event('new', 25, 10)]
    const merged = mergeLogEvents(older, head, live)
    expect(merged.map((row) => row.id)).toEqual(['new', 'head', 'tool', 'old'])
    expect(merged.find((row) => row.id === 'tool')).toMatchObject({ status: 'ok', result: 'Đã lưu' })
    expect(mergeLogEvents(merged, head, live)).toEqual(merged)
  })

  it('chỉ đếm ID mới khi cuộn xa; không đếm phát lại, cập nhật cùng ID, hoặc tải trang cũ; lên đầu xoá số', () => {
    const since = Date.parse(event('baseline', 25, 10).time)
    const first = nextUnseenEvents(new Set(['baseline']), new Set(), [event('old', 20, 1), event('baseline', 25, 10), event('fresh', 25, 20)], true, since)
    expect([...first.pending]).toEqual(['fresh'])
    const update = nextUnseenEvents(first.known, first.pending, [event('fresh', 25, 20, { status: 'ok' }), event('second', 25, 30), event('second', 25, 30)], true, since)
    expect([...update.pending]).toEqual(['fresh', 'second'])
    const top = nextUnseenEvents(update.known, update.pending, [event('third', 25, 40)], false, since)
    expect(top.pending.size).toBe(0)
    const replay = nextUnseenEvents(top.known, top.pending, [event('third', 25, 40)], true, since)
    expect(replay.pending.size).toBe(0)
  })

  it('3 lời can thiệp + 1 sự kiện engine đến lúc đang cuộn xa = đúng 4, dù mỗi lần render đưa lại toàn bộ danh sách; bài của chính mình cũng tính', () => {
    const since = Date.parse(event('baseline', 25, 10).time)
    const user = (id: string, second: number) => event(id, 25, second, { category: 'USER', agent: 'user', username: 'dat' })
    const stream = [user('u1', 20), user('u2', 22), user('u3', 26), event('w1', 25, 29, { agent: 'writer', status: 'running' })]
    let state = { known: new Set(['baseline']), pending: new Set<string>() }
    // Mỗi lần SSE đến, live.events chứa tất cả sự kiện từ đầu phiên (cộng dồn), không chỉ cái mới.
    for (let delivered = 1; delivered <= stream.length; delivered++) {
      const incoming = stream.slice(0, delivered)
      state = nextUnseenEvents(state.known, state.pending, incoming, true, since)
      state = nextUnseenEvents(state.known, state.pending, incoming, true, since) // render lại cùng dữ liệu
      expect(state.pending.size).toBe(delivered)
    }
    state = nextUnseenEvents(state.known, state.pending, [...stream.slice(0, 3), event('w1', 25, 29, { agent: 'writer', status: 'ok', finishedAt: '2026-10-25T14:31:40Z' })], true, since)
    expect([...state.pending]).toEqual(['u1', 'u2', 'u3', 'w1'])
  })

  it('khối chi tiết kỹ thuật: tham số ngắn một dòng, dài thì mỗi tham số một dòng, kết quả và dòng thử lại theo artboard', () => {
    expect(toolDetailText({ tool: 'update_outline', args: '{"from":25,"to":27,"arc":"Bản gốc"}', result: '3 chương', status: 'ok' }))
      .toBe('update_outline({ from: 25, to: 27, arc: "Bản gốc" })\n→ 3 chương')
    expect(toolDetailText({ tool: 'draft_chapter', args: '{"chapter":25,"target_words":3200,"outline":"Tri Hạ gửi lại kịch bản gốc, không ký tên"}', result: 'đang chạy · 1.840 chữ · 41s', status: 'running' }))
      .toBe('draft_chapter({\n  chapter: 25,\n  target_words: 3200,\n  outline: "Tri Hạ gửi lại kịch bản gốc, không ký tên"\n})\n→ đang chạy · 1.840 chữ · 41s')
    expect(toolDetailText({ tool: 'check_consistency', args: '{"chapter":24}', result: '429 Too Many Requests', status: 'error', retries: [{ atSec: 0, error: '429', waitSec: 8, ok: true }, { atSec: 9, error: '429', waitSec: 16, ok: false }] }))
      .toBe('check_consistency({ chapter: 24 })\n✗ 429 Too Many Requests\n↻ thử lại lần 1 sau 8s → OK\n↻ thử lại lần 2 sau 16s → chưa được')
    expect(toolDetailText({ tool: 'save_chapter', args: '', status: 'running' })).toBe('save_chapter()\n→ đang chạy…')
    expect(toolDetailText({ tool: 'save_chapter', args: '{"chapter":2', status: 'ok' })).toBe('save_chapter({"chapter":2)\n→ Không có kết quả trả về')
  })

  it('bảng nhãn·giá trị: from/to gộp thành Chương N–M, khóa kỹ thuật không hiện, chữ qua fieldLabel', () => {
    expect(toolArgRows({ args: '{"chapter":25,"target_words":3200,"instruction":"Giữ nhịp"}' })).toEqual([
      { label: 'Chương', value: '25' }, { label: 'Độ dài mục tiêu', value: '3.200 chữ' }, { label: 'Góp ý', value: 'Giữ nhịp' },
    ])
    expect(toolArgRows({ args: '{"from":22,"to":24,"arc":"Bản gốc"}' })).toEqual([
      { label: 'Chương', value: '22–24' }, { label: 'Hồi', value: 'Bản gốc' },
    ])
    expect(toolArgRows({ args: '{"replace_all":true}' })).toEqual([])
    expect(toolArgRows({ args: 'chưa phải json' })).toEqual([])
    expect(toolArgRows({})).toEqual([])
  })

  it('dòng kết quả: lỗi dịch theo bảng, JSON/đường dẫn thành “Đã xong”, chữ thường giữ nguyên', () => {
    const base = { failed: true, level: 'error' as const }
    expect(toolResultText({ ...base, result: '429 Too Many Requests', status: 'error' })).toMatch(/vượt hạn mức.*429/)
    expect(toolResultText({ ...base, result: 'ENOSPC: no space left on device', status: 'error' })).toBe('Ổ đĩa máy chủ đã đầy')
    expect(toolResultText({ failed: false, level: 'info', result: '{"ok":true,"words":1840}', status: 'ok' })).toBe('Đã xong')
    expect(toolResultText({ failed: false, level: 'info', result: 'Đang viết · 1.840 chữ', status: 'running' })).toBe('Đang viết · 1.840 chữ')
    expect(toolResultText({ ...base, result: 'Kết nối bị ngắt', status: 'error' })).toBe('Kết nối bị ngắt')
    expect(toolResultText({ ...base, status: 'error' })).toBe('Công cụ báo lỗi')
  })

  it('công cụ chưa có trong bảng vẫn hiện: nhãn chữ thường, icon code (nghiệm thu Gói J)', () => {
    const chip = toolChipText({ tool: 'pluck_moonbeam' })
    expect(chip).toBe(chip.toLocaleLowerCase('vi'))
    expect(toolIcon('pluck_moonbeam')).toBe('code')
  })
  it('giờ dòng sự kiện: hôm nay có giây, hôm qua “hôm qua HH:MM”, cũ hơn ngày/tháng', () => {
    const now = new Date(2026, 9, 25, 15, 0, 0)
    expect(logTime(new Date(2026, 9, 25, 14, 31, 40).toISOString(), now)).toBe('14:31:40')
    expect(logTime(new Date(2026, 9, 24, 21, 4, 9).toISOString(), now)).toBe('hôm qua 21:04')
    expect(logTime(new Date(2026, 9, 20, 8, 5, 0).toISOString(), now)).toBe('20/10 08:05')
    expect(logTime('không phải giờ', now)).toBe('—')
  })
  // MLog.dc.html › state=review: vào bằng ?loc=review thì mở sẵn bộ lọc “Biên tập”.
  it('state=review mở sẵn bộ lọc Biên tập, hash khác thì Tất cả', () => {
    expect(initialLogFilter('#/p/dit-dang-audio/s/01-tra-xanh-cuop-kich-ban/activity?loc=review')).toBe('editor')
    expect(initialLogFilter('#/p/dit-dang-audio/s/01-tra-xanh-cuop-kich-ban/activity')).toBe('all')
    expect(initialLogFilter('#/p/dit-dang-audio/s/01-tra-xanh-cuop-kich-ban/activity?loc=all')).toBe('all')
    expect(initialLogFilter('http://127.0.0.1:5200/p/dit-dang-audio/s/01-tra-xanh-cuop-kich-ban/activity?loc=review')).toBe('editor')
    expect(initialLogFilter('http://127.0.0.1:5200/#/p/x/s/y/activity?loc=review')).toBe('editor')
    expect(initialLogFilter('http://127.0.0.1:5200/p/x/s/y/activity')).toBe('all')
  })
})
