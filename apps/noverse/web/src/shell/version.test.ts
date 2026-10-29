import { describe, expect, it } from 'vitest'
import { buildRows, buildStamp, hasNewVersion, releaseRows, runningLine, shortCommit, versionNote, versionText } from './version'

describe('D1 › phiên bản: nhãn và thời điểm build', () => {
  it('versionText: bỏ “v” đầu như artboard “Noverse 0.9.3”, giữ nguyên dev/rỗng', () => {
    expect(versionText('v0.9.3')).toBe('0.9.3')
    expect(versionText('0.9.3')).toBe('0.9.3')
    expect(versionText('dev')).toBe('dev')
    expect(versionText(undefined)).toBe('')
  })

  it('buildStamp: RFC3339 → dd/MM/yyyy HH:mm; unknown/rỗng → rỗng', () => {
    // Giờ máy để kết quả không lệch múi giờ.
    expect(buildStamp(new Date(2026, 9, 24, 14, 2).toISOString())).toBe('24/10/2026 14:02')
    expect(buildStamp('unknown')).toBe('')
    expect(buildStamp(undefined)).toBe('')
  })

  it('shortCommit: 7 ký tự đầu; unknown/rỗng → rỗng', () => {
    expect(shortCommit('a41c9e2f4d8b7c6a5e4d3c2b1a09f8e7d6c5b4a3')).toBe('a41c9e2')
    expect(shortCommit('unknown')).toBe('')
  })

  it('runningLine: “0.9.3 · build 24/10/2026 14:02”, thiếu build thì chỉ còn số bản', () => {
    const built = new Date(2026, 9, 24, 14, 2).toISOString()
    expect(runningLine({ version: 'v0.9.3', built })).toBe('0.9.3 · build 24/10/2026 14:02')
    expect(runningLine({ version: 'v0.9.3' })).toBe('0.9.3')
  })

  it('hasNewVersion: chỉ khi máy chủ nói có và biết số bản mới', () => {
    expect(hasNewVersion({ hasUpdate: true, latestVersion: 'v0.9.4' })).toBe(true)
    expect(hasNewVersion({ hasUpdate: true, latestVersion: '' })).toBe(false)
    expect(hasNewVersion({ hasUpdate: false, latestVersion: 'v0.9.4' })).toBe(false)
  })

  it('versionNote: ghi chú nhóm “Phiên bản Noverse”', () => {
    expect(versionNote({ version: 'v0.9.3', latestVersion: 'v0.9.4', hasUpdate: true }))
      .toBe('Có bản mới 0.9.4 · bạn đang dùng 0.9.3')
    expect(versionNote({ version: 'v0.9.3', latestVersion: 'v0.9.3', hasUpdate: false }))
      .toBe('Đang dùng bản mới nhất · 0.9.3')
  })
})

describe('D1 › “Viết bằng phiên bản”: một dòng mỗi khoảng chương', () => {
  it('ba cột như artboard: bản · khoảng chương · ngày; commit đi kèm bản ghi', () => {
    const at = new Date(2026, 9, 20, 9).toISOString()
    expect(buildRows([
      { from: 1, to: 5, version: '0.9.2', commit: 'a41c9e2', at },
      { from: 6, to: 6, version: '0.9.3', commit: 'b7d1f0a', at: new Date(2026, 9, 24, 9).toISOString() },
      { from: 7, to: 8, version: '', commit: '', at: '' },
    ])).toEqual([
      { version: '0.9.2', chapters: 'Chương 1–5', commit: 'a41c9e2', date: '20/10/2026' },
      { version: '0.9.3', chapters: 'Chương 6', commit: 'b7d1f0a', date: '24/10/2026' },
      { version: '', chapters: 'Chương 7–8', commit: '', date: '—' },
    ])
  })

  it('bản ghi thiếu số chương bị bỏ, không có builds → không dòng nào', () => {
    expect(buildRows([{ from: 0, to: 3, version: '0.9.2', commit: '', at: '' }, { from: 5, to: 4, version: '0.9.2', commit: '', at: '' }])).toEqual([])
    expect(buildRows(undefined)).toEqual([])
  })
})

describe('D1 › “Có gì mới”: bản ghi release → dòng có nhãn', () => {
  it('markdown nguyên văn: tiêu đề quyết định nhãn Mới/Sửa, gạch đầu dòng thành mục', () => {
    const rows = releaseRows([
      { version: 'v0.9.4', date: '2026-10-25T10:00:00Z', notes: '## Mới\n- Xuất bản đọc cho TTS\n## Sửa\n- Ghi rõ số mục luật' },
      { version: 'v0.9.3', date: '2026-10-24T14:02:00+07:00', notes: '' },
    ], 'v0.9.3')
    expect(rows).toEqual([
      { version: '0.9.4', date: '25/10/2026', current: false, items: [
        { text: 'Xuất bản đọc cho TTS', kind: 'Mới' },
        { text: 'Ghi rõ số mục luật', kind: 'Sửa' },
      ] },
      { version: '0.9.3', date: '24/10/2026', current: true, items: [] },
    ])
  })

  it('bản không có notes vẫn hiện một dòng; máy chủ chưa trả releases → không dòng nào', () => {
    expect(releaseRows([{ version: 'v0.9.3', date: '', notes: '' }], 'v0.9.3')).toEqual([{ version: '0.9.3', date: '', current: true, items: [] }])
    expect(releaseRows(undefined, 'v0.9.3')).toEqual([])
  })
})
