// Định dạng dùng riêng màn Kênh: ngày/tháng của artboard (dd/MM), giờ hẹn
// (HH:MM), màu avatar xoay theo kênh (mỗi kênh một tint phẳng như Explore).
const day = new Intl.DateTimeFormat('vi-VN', { day: '2-digit', month: '2-digit' })
const full = new Intl.DateTimeFormat('vi-VN', { day: '2-digit', month: '2-digit', year: 'numeric' })
const time = new Intl.DateTimeFormat('vi-VN', { hour: '2-digit', minute: '2-digit', hour12: false })

const parse = (iso: string | null | undefined): Date | null => {
  if (!iso) return null
  const date = new Date(iso)
  return Number.isNaN(date.getTime()) ? null : date
}

/** "12/10" — ngày tạo kênh trong đầu trang và cột trái. */
export const fmtDay = (iso: string | null | undefined): string => {
  const date = parse(iso)
  return date ? day.format(date) : ''
}

export const fmtDate = (iso: string | null | undefined): string => {
  const date = parse(iso)
  return date ? full.format(date) : ''
}

/** "07:00" — giờ tự thử lại của nhận diện lỗi. */
export const fmtTime = (iso: string | null | undefined): string => {
  const date = parse(iso)
  return date ? time.format(date) : ''
}
