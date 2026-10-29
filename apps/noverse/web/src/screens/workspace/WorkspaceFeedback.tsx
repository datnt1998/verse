import { ApiError } from '../../api/client'
import { paths } from '../../router'
import { Banner } from '@noverse/ui'
import { translateHost } from '../../i18n/hostStrings'

/** Lỗi worker không được che khuất dữ liệu đọc trực tiếp từ gateway. */
export function WorkspaceFeedback({ error, admin, onRetry }: { error: unknown; admin: boolean; onRetry: () => void }) {
  if (!error) return null
  const payload = error instanceof ApiError && error.payload && typeof error.payload === 'object'
    ? error.payload as { needsConfig?: boolean } : null
  const message = error instanceof Error ? translateHost(error.message) : 'Không kết nối được máy chủ. Vui lòng thử lại.'
  return <Banner tone="error" title={message} action={{ label: 'Thử lại', onClick: onRetry }}>
    {admin && payload?.needsConfig === true && <a href={`#${paths.settings('providers')}`} style={{ color: 'var(--color-info)' }}>Tuỳ chỉnh › Nhà cung cấp</a>}
  </Banner>
}
