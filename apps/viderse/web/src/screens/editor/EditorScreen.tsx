// Editor — '#/editor/<projectId>' (ia-flows §5 S5). Dựng ở gói màn hình sau;
// lúc này hiện trạng thái trống của design.
import { t } from '../../i18n/vi'
import type { Route } from '../../router'
import { ScreenEmpty } from '../../components'

export function EditorScreen({ route }: { route: Extract<Route, { name: 'editor' }> }) {
  return <ScreenEmpty title={t.screens.editor.emptyTitle} hint={t.screens.editor.emptyHint} />
}
