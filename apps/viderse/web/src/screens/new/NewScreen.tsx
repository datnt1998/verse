// Video mới — '#/new' (ia-flows §5 S2 luồng Làm video). Dựng ở gói màn hình
// sau; lúc này hiện trạng thái trống của design.
import { t } from '../../i18n/vi'
import { ScreenEmpty } from '../../components'

export function NewScreen() {
  return <ScreenEmpty icon="video-add" title={t.screens.new.emptyTitle} hint={t.screens.new.emptyHint} />
}
