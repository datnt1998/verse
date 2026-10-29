// Duyệt — '#/review' (ia-flows §5 S4). Hàng chờ duyệt dựng ở gói màn hình sau;
// lúc này hiện trạng thái trống của design.
import { t } from '../../i18n/vi'
import { navigate, paths } from '../../router'
import { ActionButton, ScreenEmpty } from '../../components'

export function ReviewScreen() {
  return (
    <ScreenEmpty
      title={t.screens.review.emptyTitle}
      hint={t.screens.review.emptyHint}
      action={<ActionButton variant="primary" onClick={() => navigate(paths.workbench())}>{t.screens.review.emptyAction}</ActionButton>}
    />
  )
}

