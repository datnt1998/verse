export type ChapterMenuAction = 'read' | 'listen' | 'rewrite' | 'export' | 'copy'

export interface ChapterMenuItem {
  id: ChapterMenuAction
  label: string
  /** Tên file biểu tượng — đúng file mà artboard MReader gắn cho `.i-book/.i-headphone/.i-magic/.i-export/.i-copy`. */
  icon: string
  /** Chỉ "Nhờ viết lại…" dùng màu nhấn #DCC8F8, còn lại #EDEDEF. */
  accent?: boolean
}

/**
 * Menu ⋯ của từng chương (MReader `acItems`). Viết lại/xuất cần quyền ghi;
 * “Chép liên kết chương” là liên kết tới chương nên ai đọc được cũng chép được.
 */
export function chapterMenuItems({ writable, exporting }: { writable: boolean; exporting: boolean }): ChapterMenuItem[] {
  return [
    { id: 'read', label: 'Đọc chương', icon: 'book-1' },
    { id: 'listen', label: 'Nghe chương', icon: 'headphone' },
    ...(writable ? [
      { id: 'rewrite' as const, label: 'Nhờ viết lại…', icon: 'magic-star', accent: true },
      { id: 'export' as const, label: exporting ? 'Đang xuất chương…' : 'Xuất riêng chương này…', icon: 'export-square' },
    ] : []),
    { id: 'copy', label: 'Chép liên kết chương', icon: 'copy' },
  ]
}
