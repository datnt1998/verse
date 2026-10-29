// Khám phá — '#/explore' (ia-flows §5 S3). Quét đối thủ dựng ở gói màn hình
// sau; lúc này hiện trạng thái trống của design.
import { t } from '../../i18n/vi'
import type { Route } from '../../router'
import { ScreenEmpty } from '../../components'

export function ExploreScreen({ route }: { route: Extract<Route, { name: 'explore' }> }) {
  return <ScreenEmpty title={t.screens.explore.emptyTitle} hint={t.screens.explore.emptyHint} />
}
