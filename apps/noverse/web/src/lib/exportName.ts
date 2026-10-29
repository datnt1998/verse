// Tên tệp xuất thân thiện — thứ duy nhất người dùng thấy về tệp xuất (bất biến #7: không đường
// dẫn, không slug). Phải khớp friendlyExportName ở internal/entry/web/exports_download.go, vì
// GET …/exports/download đặt đúng tên này vào Content-Disposition.

const FALLBACK_TITLE = 'Truyện'
// eslint-disable-next-line no-control-regex
const FORBIDDEN = /[\u0000-\u001f\u007f/\\:*?"<>|]/g

/** Bỏ ký tự không hợp lệ trong tên tệp, gộp khoảng trắng, cắt dấu chấm/khoảng trắng hai đầu. */
export function sanitizeFileTitle(title: string): string {
  return title.replace(FORBIDDEN, ' ').split(/\s+/).filter(Boolean).join(' ').replace(/^[. ]+|[. ]+$/g, '')
}

/** “Trà Xanh Cướp Kịch Bản · Tập 1.epub” — không có tập thì “Trà Xanh Cướp Kịch Bản.epub”.
 *  Bản đọc TTS (D3) là tệp .txt nhưng hiện thêm “· bản đọc” trước đuôi: “<tên> · Tập 1 · bản đọc.txt”. */
export function friendlyExportName(title: string | null | undefined, volume: number | null | undefined, format: string): string {
  const clean = sanitizeFileTitle(title ?? '') || FALLBACK_TITLE
  const ext = format === 'narration'
    ? ' · bản đọc.txt'
    : format.startsWith('.') ? format : `.${format}`
  return volume && volume > 0 ? `${clean} · Tập ${volume}${ext}` : `${clean}${ext}`
}

/** Số tập suy ra từ đường dẫn tệp xuất (đoạn nào chứa tap-N / vol-N), khớp inferExportVolume phía Go. */
export function inferExportVolume(path: string): number | null {
  for (const segment of path.replace(/\\/g, '/').split('/')) {
    const match = /(?:tap|vol)[-_. ]*(\d+)/i.exec(segment)
    if (match) {
      const volume = Number.parseInt(match[1], 10)
      if (volume > 0) return volume
    }
  }
  return null
}

/** Phần mở rộng không dấu chấm, chữ thường (“a/b.EPUB” → “epub”). */
export function exportExt(path: string): string {
  const name = path.split(/[/\\]/).pop() ?? path
  const dot = name.lastIndexOf('.')
  return dot < 0 ? '' : name.slice(dot + 1).toLowerCase()
}

/**
 * Định dạng hiển thị của một tệp xuất. Bản đọc TTS cũng là tệp .txt nên nhận ra bằng hậu tố
 * `-narration` (BE: `exp.Format.FileSuffix()`, ổn định); còn lại theo đuôi tệp.
 */
export function exportFormatOf(path: string): string {
  const name = (path.split(/[/\\]/).pop() ?? path).toLowerCase()
  return /-narration\.[a-z0-9]+$/.test(name) ? 'narration' : exportExt(path)
}

/** “Bản xuất › Tập 1 › Trà Xanh Cướp Kịch Bản · Tập 1.txt” — vị trí tệp bằng khái niệm đời thường. */
export function exportLocationLabel(name: string, volume: number | null | undefined): string {
  return volume && volume > 0 ? `Bản xuất › Tập ${volume} › ${name}` : `Bản xuất › ${name}`
}
