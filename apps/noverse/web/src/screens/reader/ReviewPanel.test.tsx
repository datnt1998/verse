import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it, vi } from 'vitest'
import type { ReviewEntry, ReviewItem } from '../../api/types'
import { ReviewPanel } from './ReviewPanel'

vi.mock('../quality/QualityLink', () => ({ ReaderQualityLink: () => null }))

const item = (id: string, text: string): ReviewItem => ({ id, text, level: 'error', ref: 'Master §7', refName: '', quote: 'Trích vòng trước', fix: '' })

const entry: ReviewEntry = {
  chapter: 1, scope: 'chapter', issues: [], verdict: 'rewrite', summary: 'Đã kiểm lại',
  round: 2, pack: null, blocking: true, recheck: true,
  items: [item('ch1-old', 'Mục vòng hiện tại khác nội dung')],
  previous: [{ round: 1, verdict: 'rewrite', rule_aware: true, items: [item('ch1-old', 'Lỗi gốc vòng trước'), item('ch1-fixed', 'Lỗi đã sửa'), item('ch1-unknown', 'Lỗi chưa xác định')] }],
  recheck_results: [
    { finding_id: 'ch1-old', status: 'unfixed', note: 'Khóa vẫn còn' },
    { finding_id: 'ch1-fixed', status: 'fixed', note: 'Đã mở chốt' },
    { finding_id: 'ch1-unknown', status: 'unknown', note: 'Thiếu bằng chứng' },
  ],
}

const render = (review: ReviewEntry) => renderToStaticMarkup(<ReviewPanel chapter={1} review={{ entry: review, origin: 'chapter' }}
  plan={null} facts={null} revisions={[]} foreshadow={[]} error={null} onRetry={() => {}} writable={false} onRequestFix={() => {}} />)

describe('kết quả kiểm lại', () => {
  it('hiện nhãn và ghép chip theo finding_id của vòng lưu trữ, không ghép theo vị trí hay mục hiện tại', () => {
    const html = render(entry)
    expect(html).toContain('Kiểm lại')
    expect(html).toContain('Chưa sửa')
    expect(html).toContain('Đã sửa')
    expect(html).toContain('Chưa rõ')
    expect(html).toContain('Lỗi gốc vòng trước')
    expect(html).toContain('Lỗi đã sửa')
    expect(html).toContain('Lỗi chưa xác định')
    expect(html).toContain('Trích vòng trước')
  })
  it('không gắn nhãn kiểm lại cho review thường', () => {
    expect(render({ ...entry, recheck: false, recheck_results: [] })).not.toContain('Kiểm lại')
  })
})
