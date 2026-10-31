import { describe, expect, it } from 'vitest'
import { runCardModel } from './runCardModel'

describe('thẻ luồng viết', () => {
  it.each([
    ['novel_context', '', 'Đọc bối cảnh'], ['read_chapter', '', 'Đọc chương'],
    ['plan_chapter', '', 'Lập kế hoạch chương'], ['draft_chapter', '', 'Viết bản nháp'],
    ['edit_chapter', '', 'Gọt giũa'], ['check_consistency', '', 'Soát nhất quán'],
    ['commit_chapter', '', 'Lưu chương'], ['audit_foundation', '', 'Soát nền truyện'],
    ['revise_outline', '', 'Sửa dàn ý'], ['resolve_outline_feedback', '', 'Giữ dàn ý'],
    ['save_book', '', 'Lưu thông tin truyện'], ['save_foundation', 'premise', 'Lưu tiền đề'],
    ['save_foundation', 'characters', 'Lưu nhân vật'], ['save_foundation', 'world_rules', 'Lưu quy tắc thế giới'],
    ['save_foundation', 'outline', 'Lưu dàn ý'], ['save_foundation', 'layered_outline', 'Lưu dàn ý'],
    ['save_foundation', 'expand_arc', 'Lưu dàn ý'], ['save_foundation', 'append_volume', 'Lưu dàn ý'],
    ['save_foundation', 'update_compass', 'Cập nhật hướng đi'], ['save_foundation', 'complete_book', 'Hoàn tất truyện'],
    ['save_review', '', 'Lưu thẩm duyệt'], ['save_arc_summary', '', 'Tóm tắt cung'],
    ['save_volume_summary', '', 'Tóm tắt tập'], ['new_unknown_key', '', 'Thao tác khác'],
  ])('%s %s → %s', (key, type, name) => {
    const card = runCardModel(key, JSON.stringify({ type, chapter: 4 }), '{}')
    expect(card.name).toBe(name)
    expect(JSON.stringify(card)).not.toContain(key)
  })
  it('đọc bản nháp và lỗi không lộ khoá thô', () => {
    expect(runCardModel('read_chapter', '{"chapter":4,"source":"draft","character":"An"}', '{}').summary).toBe('chương 4 · bản nháp · lời thoại của An')
    expect(runCardModel('unknown_operation', '{}', '{}').summary).toBe('loại thao tác mới · xem chi tiết kỹ thuật')
  })
  it('dịch nhóm quy tắc thế giới đã biết, bỏ qua nhóm lạ', () => {
    const model = runCardModel('save_foundation', JSON.stringify({ type: 'world_rules', content: [
      { category: 'society', rule: 'Luật A' },
      { category: 'technology', rule: 'Luật B' },
      { category: 'unrecognized', rule: 'Luật C' },
    ] }), '{}')
    expect(model.summary).toBe('3 quy tắc · xã hội, công nghệ')
    expect(model.blocks).toContainEqual({ kind: 'list', label: 'Quy tắc', items: ['Xã hội: Luật A', 'Công nghệ: Luật B', 'Luật C'] })
  })
  it.each([
    ['novel_context', { chapter: 4 }, { premise: 'x', outline: {}, characters: [1,2], foreshadow: [1] }, 'chương 4 · tiền đề, dàn ý, 2 nhân vật, 1 cài cắm'],
    ['plan_chapter', { chapter: 4, title: 'Tên chương' }, {}, 'chương 4 · Tên chương'],
    ['draft_chapter', { chapter: 4, mode: 'write' }, { words: 3412 }, 'chương 4 · viết mới · 3.412 chữ'],
    ['edit_chapter', { chapter: 4 }, { replaced: 1 }, 'chương 4 · sửa 1 đoạn'],
    ['check_consistency', { chapter: 4 }, { world_rules: [], foreshadow: [], relations: [], aliases: [], summaries: [] }, 'chương 4 · nạp 5 nguồn đối chiếu'],
    ['commit_chapter', { chapter: 4, address_terms: [1] }, { words: 3618, foreshadow: [1,2] }, 'chương 4 · 3.618 chữ · 2 cài cắm · 1 xưng hô'],
    ['audit_foundation', {}, { ready: true, issues: [{}] }, 'sẵn sàng · 1 lưu ý'],
    ['revise_outline', { from_chapter: 5 }, { changed: [5] }, 'từ chương 5 · đổi 1 chương'],
    ['resolve_outline_feedback', {}, {}, 'không cần đổi'],
    ['save_book', { title: 'Tên', synopsis: 'Giới thiệu' }, {}, 'tên + giới thiệu'],
    ['save_foundation', { type: 'characters', content: [{ role: 'main' }, { role: 'main' }, {}] }, {}, '3 nhân vật · 2 chính'],
    ['save_foundation', { type: 'update_compass', content: { open_threads: ['a', 'b'] } }, {}, '2 tuyến còn mở'],
    ['save_review', { chapter: 3, verdict: 'accept', issues: [{}, {}] }, {}, 'chương 3 · Đạt · 2 cảnh báo'],
    ['save_arc_summary', { volume: 1, arc: 1, key_events: [1,2,3,4] }, {}, 'tập 1 · cung 1 · 4 sự kiện'],
  ])('%s tóm tắt từ dữ liệu', (key, params, result, expected) => {
    expect(runCardModel(key, JSON.stringify(params), JSON.stringify(result)).summary).toBe(expected)
  })
})
