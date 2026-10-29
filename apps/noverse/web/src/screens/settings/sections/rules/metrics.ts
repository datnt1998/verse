// D6 — thẻ “Kiểm tra 3 chương gần nhất” của Tuỳ chỉnh › Quy tắc viết (Settings.dc.html › metrics=issues).
// Mặc định chỉ hiện chỗ lệch chuẩn; chương không lệch nào ghi “Đạt mọi số đo” + số số đo đã đo.
// Mức của mỗi chỗ lệch lấy từ `severity` máy chủ gửi (tệp luật của gói ghi mức); thiếu thì “Cảnh báo”
// như trước, và `ref` (“Bộ A §4.3”) thành dòng phụ dưới câu số đo. Dòng thêm “Từ mệt nhiều nhất ·
// “x”” là một chỗ lệch nữa khi mật độ vượt ngưỡng (artboard: ““dường như” 2,4 … · tối đa 1,5”).
import type { ChapterMetric, MetricSeverity, TiredWord } from '../../../../api/types'
import { t } from '../../../../i18n/vi'
import { metricStandard, metricValue, tiredWordLimit, tiredWordName, viNumber } from '../../../../lib/metrics'

/** Một dòng của thẻ: chương mới nhất trước, mỗi mục một badge mức + câu số đo. */
export type MetricsRow = {
  chapter: number
  items: { level: MetricSeverity | 'ok'; text: string; sub: string }[]
}

/** Mức của một chỗ lệch: `severity` máy chủ gửi, thiếu thì cảnh báo (như trước đợt 3). */
export const metricLevel = (metric: Pick<ChapterMetric, 'severity'>): MetricSeverity => metric.severity ?? 'warning'

/**
 * Số đo → dòng của thẻ. Chương lệch: mỗi số đo lệch một dòng `tên · số · chuẩn` (D6: luôn có
 * đơn vị và chuẩn, không hiện điểm số trần) + dòng phụ `ref` khi máy chủ ghi mục luật.
 * Chương sạch: một dòng “Đạt mọi số đo”. Số đo không áp cho chương (`value` null) không tính là lệch.
 * Máy chủ trả chương tăng dần; thẻ đọc mới nhất trước như artboard (24 · 23 · 22).
 */
export function metricsCardRows(chapters: { n: number; metrics: ChapterMetric[]; tiredWord?: TiredWord | null }[]): MetricsRow[] {
  return [...chapters].sort((a, b) => b.n - a.n).map(({ n, metrics, tiredWord }) => {
    const off = metrics.filter((metric) => metric.value !== null && !metric.ok)
    const items: MetricsRow['items'] = off.map((metric) => ({
      level: metricLevel(metric),
      text: `${metric.name} ${metricValue(metric)} · ${metricStandard(metric.limit)}`,
      sub: metric.ref ?? '',
    }))
    if (tiredWord && !tiredWord.ok) {
      items.push({
        level: 'warning',
        text: `${tiredWordName(tiredWord.word)} ${t.metrics.tiredWordRate(viNumber(tiredWord.per1000))} · ${metricStandard(tiredWordLimit(tiredWord))}`,
        sub: '',
      })
    }
    return items.length === 0
      ? { chapter: n, items: [{ level: 'ok' as const, text: t.metrics.clean, sub: t.metrics.cleanSub(metrics.length) }] }
      : { chapter: n, items }
  })
}
