// Thư viện — '#/library' (ia-flows §5 S6). Dựng ở gói màn hình sau; lúc này
// hiện trạng thái trống của design.
import { t } from '../../i18n/vi'
import type { Route } from '../../router'
import { ScreenEmpty } from '../../components'

export function LibraryScreen({ route }: { route: Extract<Route, { name: 'library' }> }) {
  return <ScreenEmpty icon="gallery" title={t.screens.library.emptyTitle} hint={t.screens.library.emptyHint} />
}
