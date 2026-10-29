// Logic thuần của Tuỳ chỉnh › Xuất tự động (Settings.dc.html sec export).
import { friendlyExportName } from '../../../lib/exportName'

export type ExportFormat = 'txt' | 'epub' | 'narration'

/** Thứ tự cố định như artboard: Văn bản (.txt) rồi Ebook (.epub). */
export const EXPORT_FORMATS: { id: ExportFormat; label: string }[] = [
  { id: 'txt', label: 'Văn bản (.txt)' },
  { id: 'epub', label: 'Ebook (.epub)' },
]

/**
 * Bật/tắt một định dạng, giữ thứ tự cố định.
 * Định dạng màn này không hiện (vd `narration` bật từ A3) vẫn được giữ, không bị xoá khi bấm chip khác.
 */
export function toggleFormat(formats: readonly ExportFormat[], format: ExportFormat, on: boolean): ExportFormat[] {
  const next = new Set(formats)
  if (on) next.add(format)
  else next.delete(format)
  const shown = EXPORT_FORMATS.map((entry) => entry.id).filter((id) => next.has(id))
  const hidden = [...next].filter((id) => !EXPORT_FORMATS.some((entry) => entry.id === id))
  return [...shown, ...hidden]
}

/** Ví dụ tên tệp bằng lời (không đường dẫn): “Trà Xanh Cướp Kịch Bản · Tập 1.epub” — ưu tiên .epub khi có chọn. */
export function exampleFileName(title: string, formats: readonly ExportFormat[]): string {
  return friendlyExportName(title, 1, formats.includes('epub') ? 'epub' : 'txt')
}
