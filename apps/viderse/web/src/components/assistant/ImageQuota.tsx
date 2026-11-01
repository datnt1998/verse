// Số ảnh đã vẽ trên hạn mức của tài khoản vẽ ảnh (cuối hàng phím tắt, artboard
// Main). Nguồn là `quota` của /api/ai/providers (nguồn quota duy nhất, D7): ưu
// tiên ChatGPT (theo ngày), không có thì Grok (theo tuần). Không có quota → ẩn.
import * as stylex from '@stylexjs/stylex'
import { Icon } from '@noverse/ui'
import { listProviders, type AiProvider } from '../../api/ai'
import { keys } from '../../data/keys'
import { useResource } from '../../data/useResource'
import { assistantCopy as copy } from '../../i18n/assistant'
import { assistantStyles as styles } from './assistant.stylex'

const DRAWERS = ['chatgpt', 'grok'] as const

export function imageQuotaOf(providers: readonly AiProvider[] | undefined): { text: string; full: boolean } | null {
  for (const kind of DRAWERS) {
    const provider = providers?.find((item) => item.kind === kind && item.quota && item.quota.limit > 0)
    if (!provider?.quota) continue
    const { used, limit, period } = provider.quota
    return { text: copy.quota.text(provider.label, used, limit, period), full: used >= limit }
  }
  return null
}

export function ImageQuota() {
  const providers = useResource(keys.aiProviders(), ({ signal }) => listProviders(signal))
  const quota = imageQuotaOf(providers.data)
  if (!quota) return null
  return <span title={copy.quota.title} {...stylex.props(styles.quota)}>
    <Icon name="gallery" size={14} xstyle={quota.full ? styles.quotaFull : styles.quotaOk} />{quota.text}
  </span>
}
