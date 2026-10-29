// Tạo kênh — '#/channels/new'. Luồng tạo kênh dựng ở gói màn hình sau;
// lúc này hiện trạng thái trống của design.
import { t } from '../../i18n/vi'
import type { Route } from '../../router'
import { ScreenEmpty } from '../../components'

export function ChannelNewScreen({ route }: { route: Extract<Route, { name: 'channel-new' }> }) {
  return <ScreenEmpty title={t.screens.channelNew.emptyTitle} hint={t.screens.channelNew.emptyHint} />
}
