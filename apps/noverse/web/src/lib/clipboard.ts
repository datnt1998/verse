/**
 * Chép chữ vào clipboard, chạy được cả khi trang mở qua HTTP thường (điện thoại vào máy nhà qua LAN/Tailscale):
 * ở ngữ cảnh không bảo mật Safari/Chrome không có `navigator.clipboard`, nên dùng ô văn bản ẩn + `execCommand('copy')`
 * ngay trong cử chỉ bấm. Thất bại → lỗi tiếng Việt, không lộ lỗi thô của trình duyệt.
 */
export const COPY_FAILED = 'Không chép được — trình duyệt chặn quyền ghi bộ nhớ tạm. Hãy chọn và chép thủ công.'

export async function copyText(text: string): Promise<void> {
  if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return
    } catch {
      // rơi xuống cách dự phòng bên dưới
    }
  }
  if (!legacyCopy(text)) throw new Error(COPY_FAILED)
}

function legacyCopy(text: string): boolean {
  if (typeof document === 'undefined') return false
  const area = document.createElement('textarea')
  area.value = text
  area.setAttribute('readonly', '')
  // Ngoài màn hình, cỡ chữ 16px để iOS không phóng to trang khi chọn.
  area.style.cssText = 'position:fixed;top:0;left:-9999px;opacity:0;font-size:16px'
  document.body.appendChild(area)
  const active = document.activeElement as HTMLElement | null
  area.select()
  area.setSelectionRange(0, text.length)
  let ok = false
  try { ok = document.execCommand('copy') } catch { ok = false }
  area.remove()
  active?.focus?.({ preventScroll: true })
  return ok
}
