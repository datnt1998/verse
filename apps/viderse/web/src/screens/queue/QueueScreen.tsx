// Hàng chờ — '#/queue'. Bảng hàng chờ dựng ở gói màn hình sau; lúc này hiện
// trạng thái trống của design.
import { t } from '../../i18n/vi'
import { navigate, paths } from '../../router'
import { ActionButton } from '../../components'
import { ScreenEmpty } from '../../components'

export function QueueScreen() {
  return (
    <ScreenEmpty
      title={t.screens.queue.emptyTitle}
      hint={t.screens.queue.emptyHint}
      action={<ActionButton variant="primary" onClick={() => navigate(paths.new())}>{t.screens.queue.emptyAction}</ActionButton>}
    />
  )
}
