import { isNeedsConfig } from '../api/needsConfig'
import { useSession } from '../api/session'
import { navigate, paths } from '../router'

/**
 * Thiếu cấu hình model không tự hết khi thử lại: quản trị viên đi tới Tuỳ chỉnh › Nhà cung cấp
 * (trên điện thoại là trang “Tiếp tục trên máy tính”, 18-mobile), người khác biết nhờ ai.
 * Không render gì nếu lỗi không phải needsConfig.
 */
export function NeedsConfigHint({ error }: { error: unknown }) {
  const { user } = useSession()
  if (!isNeedsConfig(error)) return null
  return user?.role === 'admin'
    ? <button type="button" className="m-config-hint" onClick={() => navigate(paths.settings('providers'))}>Mở Tuỳ chỉnh › Nhà cung cấp (làm trên máy tính)</button>
    : <span className="m-config-hint">Chỉ quản trị viên thiết lập được nhà cung cấp AI — hãy nhờ quản trị viên của máy chủ này.</span>
}
