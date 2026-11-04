/** Bản một cột (dưới 768px) trên màn chạm: mọi nút của Khám phá cao ≥ 44px. */
export const TOUCH_COLUMN = '(max-width: 767px) and (pointer: coarse)'

/** Cao nút chạm tối thiểu của bản một cột. */
export const TOUCH_TARGET = 44

/** Bề rộng tối đa của lưới Khám phá trên màn rộng — bằng `--vv-wide-grid` (viderse.stylex.ts). */
export const GRID_MAX_W = 1760

/** Sao chép một link — navigator.clipboard, fallback execCommand. Trả false
 *  khi trình duyệt chặn (toast lỗi ở caller). */
export async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    try {
      const el = document.createElement('textarea')
      el.value = text
      el.style.position = 'fixed'
      el.style.opacity = '0'
      document.body.append(el)
      el.select()
      const ok = document.execCommand('copy')
      el.remove()
      return ok
    } catch {
      return false
    }
  }
}
