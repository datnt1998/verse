// Cài đặt template — '#/templates/<id>'. Dựng ở gói màn hình sau; lúc này
// hiện trạng thái trống của design.
import { t } from '../../i18n/vi'
import type { Route } from '../../router'
import { ScreenEmpty } from '../../components'

export function TemplateScreen({ route }: { route: Extract<Route, { name: 'template' }> }) {
  return <ScreenEmpty title={t.screens.template.emptyTitle} hint={t.screens.template.emptyHint} />
}
