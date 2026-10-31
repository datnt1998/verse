// Thanh chọn nhiều: chọn kênh đích trước khi xếp các video đã chọn.
import { Icon, toast, usePopup } from '@noverse/ui'
import { vi as t } from '../../i18n/vi'
import { ActionButton } from '../../components'
import type { Video } from '../../api/rivals'
import { copyText } from './VideoGrid'

export function BulkBar({ count, selected, onTop, onClear, onExit, channels, onMake, busy = false }: {
  count: number
  selected: readonly Video[]
  onTop: (n: number) => void
  onClear: () => void
  onExit: () => void
  channels: readonly { id: string; name: string }[]
  onMake: (channelId: string) => void
  busy?: boolean
}) {
  const tt = t.screens.explore
  const { root, open, setOpen } = usePopup()
  const copyLinks = async () => {
    const urls = selected.map((v) => v.url)
    const ok = await copyText(urls.join('\n'))
    if (ok) toast.ok(tt.copied)
    else toast.err(tt.copyFailed)
  }
  return (
    <div className="vx-bulk">
      <div className="vx-bulk__bar" role="region" aria-label={tt.bulkRegion}>
        <span className="vx-bulk__count num">{tt.selected(count)}</span>
        <span className="vx-bulk__quick">
          <button type="button" onClick={() => onTop(10)}>{tt.top10}</button>
          <button type="button" onClick={() => onTop(20)}>{tt.top20}</button>
          <button type="button" onClick={onClear}>{tt.deselect}</button>
        </span>
        <ActionButton className="vx-bulk__copy" size="md" icon={<Icon name="copy" size={15} />} onClick={copyLinks}>{tt.copyLink}</ActionButton>
        <div ref={root} className="vv-switcher-root">
          <ActionButton className="vx-bulk__make" variant="primary" size="md"
            lockedReason={channels.length ? undefined : tt.lockedReason}
            disabled={busy}
            aria-haspopup="menu" aria-expanded={open}
            icon={<Icon name="clock" size={15} />}
            onClick={() => setOpen(!open)}>
            {channels[0] ? tt.bulkMakeFor(count, channels[0].name) : tt.bulkMake(count)}
            <Icon name="arrow-d" size={13} aria-hidden />
          </ActionButton>
          {open && <div role="menu" aria-label={tt.makeChannel} className="vv-switcher__menu vx-bulk__menu">
            {channels.map((channel) => <button type="button" role="menuitem" className="vv-switcher__item" key={channel.id}
              onClick={() => { setOpen(false); onMake(channel.id) }}>
              {tt.bulkMakeFor(count, channel.name)}
            </button>)}
          </div>}
        </div>
        <button type="button" className="vx-iconbtn" aria-label={tt.exitSelect} onClick={onExit}
          style={{ width: 'var(--vv-s-34)', height: 'var(--vv-s-34)' }}>
          <Icon name="close" size={16} />
        </button>
      </div>
    </div>
  )
}
