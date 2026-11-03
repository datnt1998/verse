// SuggestionCard (04-components) — Main `agent=suggest` (compact, cột trái) và Bản tin (đủ).
// Một nút chính, Bỏ qua im lặng, "Vì sao?" mở lý do ngay trong thẻ. Chỉ bản đủ có menu
// "Đừng đề xuất kiểu này".
import { useId, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Icon } from '@noverse/ui'
import { ResponsiveActionMenu } from '../responsive-overlays'
import type { Suggestion } from '../../api/suggestions'
import { t } from '../../i18n/learn'
import { ActionButton } from '../ActionButton'
import { ASSISTANT_INK } from '../agents'
import { Orb } from '../Orb'
import { ReasonPopover } from '../reason/ReasonPopover'
import { suggestStyles as s } from './suggestions.stylex'

const c = t.learn.suggest

export function SuggestionCard({ suggestion, compact = false, busy = false, offline = false, notice, onAccept, onDismiss, onNever }: {
  suggestion: Suggestion
  compact?: boolean
  busy?: boolean
  offline?: boolean
  notice?: string
  onAccept: () => void
  onDismiss: () => void
  onNever?: () => void
}) {
  const [why, setWhy] = useState(false)
  const whyId = useId()
  const lock = busy || offline
  return <article aria-label={c.who(suggestion.agent)} aria-busy={busy || undefined} {...stylex.props(s.card, compact && s.cardCompact)}>
    <span {...stylex.props(s.orb)}><Orb of={suggestion.agent} size={compact ? 18 : 22} /></span>
    <span {...stylex.props(s.who, s.ink(ASSISTANT_INK[suggestion.agent]))}>{c.who(suggestion.agent)}</span>
    <div {...stylex.props(s.title, compact && s.titleCompact)} title={suggestion.title}>{suggestion.title}</div>
    <div {...stylex.props(s.evidence)}>
      <span {...stylex.props(s.evidenceText)}>{suggestion.evidence}</span>
      <button type="button" aria-expanded={why} aria-controls={whyId} {...stylex.props(s.why, why && s.whyOpen)} onClick={() => setWhy(!why)}>
        <Icon name="info" size={compact ? 12 : 13} />{c.why}
      </button>
    </div>
    {why && <div id={whyId} {...stylex.props(s.reason)}><ReasonPopover reason={suggestion.reason} label={c.why} /></div>}
    {notice && <div role="alert" {...stylex.props(s.notice)}>{notice}</div>}
    <div {...stylex.props(s.actions)}>
      <ActionButton variant="primary" size="sm" type="button" className={stylex.props(s.primary).className} disabled={lock}
        disabledReason={offline ? c.offlineAction : undefined} onClick={onAccept}>{suggestion.primary.label}</ActionButton>
      <ActionButton variant="quiet" size="sm" type="button" disabled={lock} onClick={onDismiss}>{c.dismiss}</ActionButton>
      {!compact && onNever && <span {...stylex.props(s.more)}>
        <ResponsiveActionMenu alwaysVisible label={c.more} items={[{ id: 'never', label: c.never, disabled: lock, onSelect: onNever }]} />
      </span>}
    </div>
  </article>
}
