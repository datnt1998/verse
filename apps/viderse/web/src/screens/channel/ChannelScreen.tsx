// Kênh — '#/channels/<id>' (ia-flows §5 S1). Trang kênh dựng ở gói màn hình
// sau; lúc này hiện trạng thái trống của design.
import { t } from '../../i18n/vi'
import { navigate, paths, type Route } from '../../router'
import { ActionButton } from '../../components'
import { ScreenEmpty } from '../../components'

export function ChannelScreen({ route }: { route: Extract<Route, { name: 'channel' }> }) {
  return (
    <ScreenEmpty
      icon="user"
      title={t.screens.channel.emptyTitle}
      hint={t.screens.channel.emptyHint}
      action={<ActionButton variant="primary" onClick={() => navigate(paths.new())}>{t.screens.channel.emptyAction}</ActionButton>}
    />
  )
}
