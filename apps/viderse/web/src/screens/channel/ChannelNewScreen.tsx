import * as stylex from '@stylexjs/stylex'
import { channelStyles } from './channel.stylex'
// '#/channels/new?step=create|done' (W2 · artboard CreateChannel, và Onboard
// step=create|done khi là lần tạo kênh đầu tiên). Máy cài xong mà chưa có kênh
// nào thì AppShell bỏ nav và màn này tự vẽ đầu Onboard "Viderse · Lần đầu mở";
// Huỷ rời khung và ghi cờ phiên để không bị đưa lại (không lặp người dùng).
import { Icon } from '@noverse/ui'
import { invalidate } from '@noverse/ui/data'
import { LensLogo } from '../../components'
import { t } from '../../i18n/vi'
import { navigate, paths, type ChannelNewStep, type Route } from '../../router'
import { useResource } from '../../data/useResource'
import { keys } from '../../data/keys'
import { listChannels } from '../../api/channels'
import { dismissOnboard, onboardDismissed, setLastCreatedChannel, lastCreatedChannel } from '../../lib/onboard'
import { CreateStep } from './CreateStep'
import { DoneStep } from './DoneStep'

export function ChannelNewScreen({ route }: { route: Extract<Route, { name: 'channel-new' }> }) {
  const step: ChannelNewStep = route.step ?? 'create'
  const channels = useResource(keys.channels(), ({ signal }) => listChannels(signal))
  const list = channels.data?.channels ?? []
  const firstRun = step === 'create' && !onboardDismissed() && channels.data !== undefined && list.length === 0
  const tt = t.channels

  const leaveOnboard = () => {
    if (firstRun) dismissOnboard()
  }

  const cancel = () => {
    leaveOnboard()
    navigate(list[0] ? paths.channel(list[0].id) : paths.workbench())
  }

  const created = (id: string) => {
    dismissOnboard()
    setLastCreatedChannel(id)
    void invalidate(keys.channels())
    navigate(paths.channelNew('done'), true)
  }

  return (
    <div style={{ flex: '1 1 auto', minHeight: 0, display: 'flex', flexDirection: 'column' }}>
      {firstRun && (
        <header className={`vch-onboardhead ${stylex.props(channelStyles.onboardhead).className}`}>
          <LensLogo size={26} />
          <span className={`vch-onboardhead__brand ${stylex.props(channelStyles.onboardheadBrand).className}`}>{tt.onboard.brand}</span>
          <span className={`vch-onboardhead__first ${stylex.props(channelStyles.onboardheadFirst).className}`}>{tt.onboard.firstRun}</span>
          <a className={`vch-onboardhead__adv ${stylex.props(channelStyles.onboardheadAdv).className}`} href={`#${paths.settings()}`}>
            <Icon name="setting" size={15} />
            {tt.onboard.advanced}
          </a>
        </header>
      )}
      {step === 'done'
        ? <DoneStep channelId={lastCreatedChannel()} onLeave={leaveOnboard} />
        : <CreateStep firstRun={firstRun} onCancel={cancel} onCreated={created} />}
    </div>
  )
}

export default ChannelNewScreen
