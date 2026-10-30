// Định dạng số/đường dẫn của màn Cài đặt — số kiểu Việt Nam (81,1 GB), đường
// dẫn mono cắt giữa, giờ phút cho các thẻ.

const num = new Intl.NumberFormat('vi-VN', { maximumFractionDigits: 1 })

/** Byte → chuỗi GB kiểu Việt Nam (“86,4”). MB cho giá trị nhỏ hơn 1 GB. */
export const fmtGB = (bytes: number): string =>
  bytes >= 1e9 ? `${num.format(bytes / 1e9)} GB` : `${num.format(bytes / 1e6)} MB`

/** Như fmtGB nhưng chỉ trả phần số + đơn vị gộp — dùng trong câu có sẵn “GB”. */
export const fmtBytes = (bytes: number): string =>
  bytes >= 1e9 ? `${num.format(bytes / 1e9)} GB` : `${num.format(bytes / 1e6)} MB`

/** RFC3339 → “HH:mm” giờ máy. Chuỗi không đọc được trả nguyên. */
export const fmtTime = (iso: string | undefined | null): string => {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit' })
}

/** RFC3339 → “dd/mm” cho mốc tự xoá. */
export const fmtDay = (iso: string | undefined | null): string => {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  // Ghim “dd/mm”: ICU có máy trả “6/11” hay gạch ngang, artboard luôn “04/11”.
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getDate())}/${pad(d.getMonth() + 1)}`
}

/** Đường dẫn → cắt giữa bằng “…” khi dài; đường dẫn đầy đủ luôn nằm ở title. */
export const midTrunc = (path: string, keep = 26): string => {
  if (path.length <= keep * 2 + 4) return path
  return `${path.slice(0, keep)}…${path.slice(-keep)}`
}

/** Giây còn lại → “khoảng N phút” (đủ mờ cho câu ETA). */
export const fmtEta = (sec: number | null | undefined): string => {
  if (sec == null || sec <= 0) return ''
  const m = Math.max(1, Math.round(sec / 60))
  return `${m} phút`
}

/** Đổi chỗ giữ chỗ {k} trong chuỗi copy i18n. */
export const fill = (tpl: string, vars: Record<string, string | number>): string =>
  tpl.replace(/\{(\w+)\}/g, (_, k: string) => (k in vars ? String(vars[k]) : `{${k}}`))
