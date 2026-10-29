import { describe, expect, it } from 'vitest'
import type { ChapterMetric } from '../../../../api/types'
import { metricsCardRows } from './metrics'

const metric = (key: string, name: string, value: number | null, unit: string, limit: string, ok: boolean): ChapterMetric =>
  ({ key, name, value, unit, limit, ok })

/** Chín số đo của D6 (22-brief-2510.md câu 16) — tên và đơn vị đúng hợp đồng đợt 3. */
const allOk = (): ChapterMetric[] => [
  metric('thoai', 'Tỷ lệ thoại', 52, '%', '40–55%', true),
  metric('cau_dai_nhat', 'Câu quá 40 tiếng', 0, 'câu', '0', true),
  metric('doan_dai_nhat', 'Đoạn quá 90 tiếng', 0, 'đoạn', '0', true),
  metric('quang_khong_thoai', 'Quãng không thoại quá 250 tiếng', 0, 'quãng', '0', true),
  metric('tu_thu_tuc', 'Từ thủ tục', 4, 'từ', '≤ 10', true),
  metric('tu_tieng_anh', 'Từ tiếng Anh dễ đọc sai', 0, 'từ', '0', true),
  metric('ngay_thu', '“ngày thứ” + số', 0, 'dòng', '0', true),
  metric('cum_lap', 'Cụm 5 tiếng lặp lại', 0, 'cụm', '0', true),
  metric('trung_mo_dau', 'Mở đầu trùng truyện cũ', null, '%', '≤ 20% · chỉ đo chương 1', true),
]

describe('D6 — thẻ “Kiểm tra 3 chương gần nhất”', () => {
  it('mới nhất trước, chương lệch chỉ hiện số đo lệch kèm mức và mục luật', () => {
    const rows = metricsCardRows([
      { n: 3, metrics: allOk() },
      {
        n: 5,
        metrics: allOk().map((item) => item.key === 'thoai'
          ? { ...item, value: 34, ok: false, severity: 'warning' as const, ref: 'Master §13' }
          : item.key === 'quang_khong_thoai'
            ? { ...item, value: 332, ok: false, severity: 'error' as const, ref: 'Master §5' }
            : item),
      },
      { n: 4, metrics: allOk() },
    ])
    expect(rows.map((row) => row.chapter)).toEqual([5, 4, 3])
    expect(rows[0].items).toEqual([
      { level: 'warning', text: 'Tỷ lệ thoại 34% · chuẩn 40–55%', sub: 'Master §13' },
      { level: 'error', text: 'Quãng không thoại quá 250 tiếng 332 quãng · chuẩn 0', sub: 'Master §5' },
    ])
  })

  it('máy chủ không ghi mức thì chỗ lệch vẫn là Cảnh báo, không có dòng phụ', () => {
    const rows = metricsCardRows([{
      n: 4,
      metrics: allOk().map((item) => item.key === 'doan_dai_nhat' ? { ...item, value: 2, ok: false, detail: 'dài nhất 96 tiếng' } : item),
    }])
    expect(rows[0].items).toEqual([{ level: 'warning', text: 'Đoạn quá 90 tiếng 2 đoạn · chuẩn 0', sub: '' }])
  })

  it('chương không lệch nào: một dòng Đạt + chín số đo', () => {
    const [row] = metricsCardRows([{ n: 23, metrics: allOk() }])
    expect(row.items).toEqual([{ level: 'ok', text: 'Đạt mọi số đo', sub: '9 số đo · 0 lệch' }])
  })

  it('số đo không áp cho chương (value null) không tính là lệch', () => {
    const [row] = metricsCardRows([{
      n: 2,
      metrics: allOk().map((item) => item.key === 'trung_mo_dau' ? { ...item, value: null, ok: false } : item),
    }])
    expect(row.items.map((item) => item.level)).toEqual(['ok'])
  })

  it('từ mệt vượt ngưỡng là một dòng lệch nữa, đứng sau các số đo', () => {
    const [row] = metricsCardRows([{
      n: 24,
      metrics: allOk(),
      tiredWord: { word: 'dường như', per1000: 2.4, limit: 1.5, ok: false },
    }])
    expect(row.items).toEqual([{
      level: 'warning',
      text: 'Từ mệt nhiều nhất · “dường như” 2,4 lần trên 1.000 tiếng · tối đa 1,5',
      sub: '',
    }])
  })

  it('từ mệt trong ngưỡng thì chương vẫn sạch', () => {
    const [row] = metricsCardRows([{
      n: 23,
      metrics: allOk(),
      tiredWord: { word: 'khẽ', per1000: 0.4, limit: 2, ok: true },
    }])
    expect(row.items.map((item) => item.level)).toEqual(['ok'])
  })

  it('không có số đo thì không có dòng nào', () => {
    expect(metricsCardRows([])).toEqual([])
  })
})
