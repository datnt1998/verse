// Cài đặt — '#/settings' (ia-flows §5 S7). Dựng ở gói màn hình sau; lúc này
// hiện trạng thái trống của design.
import { t } from '../../i18n/vi'
import type { Route } from '../../router'
import { ScreenEmpty } from '../../components'

export function SettingsScreen({ route }: { route: Extract<Route, { name: 'settings' }> }) {
  return <ScreenEmpty title={t.screens.settings.emptyTitle} hint={t.screens.settings.emptyHint} />
}
