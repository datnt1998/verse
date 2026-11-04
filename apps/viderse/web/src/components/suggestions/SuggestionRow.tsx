// Một dòng Đề xuất trong nhóm Chờ bạn (V37). Toàn dòng là việc chính; nút × bỏ qua.
// Không còn thẻ, "Vì sao?", menu "Đừng đề xuất kiểu này" hay nút nhận riêng.
import * as stylex from '@stylexjs/stylex'
import { Icon } from '@noverse/ui'
import type { Suggestion } from '../../api/suggestions'
import { t } from '../../i18n/learn'
import { ASSISTANT_INK } from '../agents'
import { Orb } from '../Orb'
import { suggestStyles as s } from './suggestions.stylex'

const c = t.learn.suggest

export function SuggestionRow({ suggestion, busy = false, offline = false, notice, onAccept, onDismiss }: {
  suggestion: Suggestion
  busy?: boolean
  offline?: boolean
  notice?: string
  onAccept: () => void
  onDismiss: () => void
}) {
  const lock = busy || offline
  return <div {...stylex.props(s.row)}>
    <span aria-hidden="true" {...stylex.props(s.orb)}><Orb of={suggestion.agent} size={18} /></span>
    <button type="button" aria-busy={busy || undefined} disabled={lock}
      title={`${c.who(suggestion.agent)} · ${suggestion.title} · ${suggestion.evidence}`}
      onClick={onAccept} {...stylex.props(s.rowMain)}>
      <span {...stylex.props(s.title)} title={suggestion.title}>{suggestion.title}</span>
      <span {...stylex.props(s.evidence)}><b {...stylex.props(s.ink(ASSISTANT_INK[suggestion.agent]))}>{c.who(suggestion.agent)}</b>{' · '}{suggestion.evidence}</span>
      {notice && <span role="alert" {...stylex.props(s.notice)}>{notice}</span>}
    </button>
    <button type="button" aria-label={c.dismiss} title={offline ? c.offlineAction : c.dismiss}
      disabled={lock} onClick={onDismiss} {...stylex.props(s.skip)}>
      <Icon name="close" size={13} />
    </button>
  </div>
}
