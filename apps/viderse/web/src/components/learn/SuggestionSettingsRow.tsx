// Hàng "Đề xuất của trợ lý" trong Cài đặt thông báo › Trợ lý chủ động. Công tắc tổng bật/tắt cả
// 7 loại (mọi kênh); "Chọn loại" mở từng loại; loại tắt riêng cho một kênh (từ “Đừng đề xuất kiểu
// này”) hiện dạng danh sách có “Bật lại”. Toàn bộ nằm trong `suggestions.off` của cài đặt thông báo.
import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { SUGGESTION_KINDS, type SuggestionKind } from '../../api/suggestions'
import { t } from '../../i18n/vi'
import { ActionButton } from '../ActionButton'
import { learnStyles as s } from './learn.stylex'

type Off = { channelId: string | null; kind: SuggestionKind }
const c = t.learn.settings

export function SuggestionSettingsRow({ off, channelNames, offline, onChange, renderSwitch }: {
  off: Off[]
  channelNames: Record<string, string>
  offline: boolean
  onChange: (next: Off[]) => void
  renderSwitch: (label: string, checked: boolean, onChange: (next: boolean) => void) => React.ReactNode
}) {
  const [open, setOpen] = useState(false)
  const globallyOff = (kind: SuggestionKind) => off.some((item) => item.channelId === null && item.kind === kind)
  const onKinds = SUGGESTION_KINDS.filter((kind) => !globallyOff(kind))
  const perChannel = off.filter((item) => item.channelId !== null)
  const setKind = (kind: SuggestionKind, on: boolean) => onChange(on ? off.filter((item) => !(item.channelId === null && item.kind === kind)) : [...off, { channelId: null, kind }])
  const setAll = (on: boolean) => onChange(on ? off.filter((item) => item.channelId !== null) : [...off.filter((item) => item.channelId !== null), ...SUGGESTION_KINDS.map((kind) => ({ channelId: null, kind }))])
  const summary = c.summary(onKinds.length, SUGGESTION_KINDS.length, SUGGESTION_KINDS.filter(globallyOff).map((kind) => c.kinds[kind]))
  return <>
    <div {...stylex.props(s.setRow)}>
      <span {...stylex.props(s.setText)}>{c.row}<span {...stylex.props(s.setDesc)}>{summary}</span></span>
      <ActionButton variant="quiet" size="sm" type="button" aria-expanded={open} onClick={() => setOpen(!open)}>{open ? c.done : c.choose}</ActionButton>
      {renderSwitch(c.row, onKinds.length > 0, setAll)}
    </div>
    {open && <>
      {SUGGESTION_KINDS.map((kind) => <div key={kind} {...stylex.props(s.setRow, s.setSub)}>
        <span {...stylex.props(s.setText)}>{c.kinds[kind]}</span>
        {renderSwitch(c.kindSwitch(c.kinds[kind]), !globallyOff(kind), (on) => setKind(kind, on))}
      </div>)}
      {perChannel.map((item) => <div key={`${item.channelId}-${item.kind}`} {...stylex.props(s.setRow, s.setSub)}>
        <span {...stylex.props(s.setText)}>{c.offOne(c.kinds[item.kind], item.channelId ? channelNames[item.channelId] ?? item.channelId : c.everyChannel)}</span>
        <ActionButton variant="quiet" size="sm" type="button" disabled={offline} onClick={() => onChange(off.filter((other) => other !== item))}>{c.turnOn}</ActionButton>
      </div>)}
    </>}
  </>
}
