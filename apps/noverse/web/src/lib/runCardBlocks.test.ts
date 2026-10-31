import { describe, expect, it } from 'vitest'
import { runCardModel } from './runCardModel'

const model = (key: string, params: unknown, result: unknown = {}) => runCardModel(key, JSON.stringify(params), JSON.stringify(result))

describe('thân thẻ theo loại thao tác', () => {
  it('đối chiếu đoạn cũ và mới, rồi hiển thị phạm vi', () => {
    expect(model('edit_chapter', { old_string: 'Đoạn cũ', new_string: 'Đoạn mới', replace_all: false }).blocks).toEqual([
      { kind: 'diff', old: 'Đoạn cũ', next: 'Đoạn mới' },
      { kind: 'kv', items: [['Phạm vi', 'một chỗ']] },
    ])
  })
  it('hiển thị điểm và cảnh báo từ các trường dữ liệu thẩm duyệt', () => {
    const blocks = model('save_review', { chapter: 3, verdict: 'accept', dimensions: [{ dimension: 'Nhịp', score: 85 }], issues: [{ severity: 'warning', rule: 'Master §13', description: 'Thiếu thoại', suggestion: 'Thêm đối đáp' }] }).blocks
    expect(blocks).toContainEqual({ kind: 'scores', items: [['Nhịp', '85']] })
    expect(blocks).toContainEqual({ kind: 'issues', items: [{ level: 'w', ref: 'Master §13', text: 'Thiếu thoại', fix: 'Thêm đối đáp' }] })
  })
  it('hiển thị chip móc cuối, nhân vật, cài cắm và xưng hô thật khi lưu chương', () => {
    const blocks = model('commit_chapter', { chapter: 4, hook_type: 'lat_thong_tin', characters: ['An'], foreshadow_updates: [{ action: 'plant', description: 'Một bức thư' }], address_terms: [{ from: 'An', to: 'Bình', calls: 'chị', from_chapter: 4 }] }, { words: 3618 }).blocks
    expect(blocks).toContainEqual({ kind: 'kv', items: [['Chương', '4'], ['Số chữ', '3.618'], ['Kiểu móc', 'lật thông tin', 'b']] })
    expect(blocks).toContainEqual({ kind: 'cast', names: ['An'] })
    expect(blocks).toContainEqual({ kind: 'tags', label: 'Chi tiết cài cắm', items: [['Đã gieo', 'Một bức thư', 'b']] })
    expect(blocks).toContainEqual({ kind: 'kv', items: [['Xưng hô mới', 'An → Bình: “chị” từ chương 4']] })
  })
})
