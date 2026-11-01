// Ba nơi hiện Đề xuất: hàng nút đầu cột trái Bàn dựng (Bản tin + Đề xuất N), nhóm thẻ gọn
// trong cột trái (≤3 + “Xem cả N trong Bản tin”) và mục “Đề xuất · 3 trong 5” của Bản tin.
// Cả ba đọc chung một khoá cache nên Bỏ qua ở đâu cũng biến mất ở mọi nơi.
import * as stylex from '@stylexjs/stylex'
import { Icon } from '@noverse/ui'
import { t } from '../../i18n/learn'
import { ActionButton } from '../ActionButton'
import { Orb } from '../Orb'
import { SuggestionCard } from './SuggestionCard'
import { suggestStyles as s } from './suggestions.stylex'
import { useSuggestions } from './useSuggestions'

const c = t.learn.suggest

export function SuggestionToggles({ recapOpen, suggestOpen, count, onRecap, onToggle }: {
  recapOpen: boolean
  suggestOpen: boolean
  count: number
  onRecap: () => void
  onToggle: () => void
}) {
  return <div {...stylex.props(s.toggles)}>
    <ActionButton variant="outline" size="sm" type="button" aria-pressed={recapOpen} className={stylex.props(s.toggle, recapOpen && s.toggleOn).className}
      icon={<Icon name="doc" size={14} />} onClick={onRecap}>{c.recapButton}</ActionButton>
    <ActionButton variant="outline" size="sm" type="button" aria-expanded={suggestOpen} className={stylex.props(s.toggle, suggestOpen && s.toggleOn).className}
      icon={<span aria-hidden="true" {...stylex.props(s.orbs)}><Orb of="scout" size={10} /><span {...stylex.props(s.orbStack)}><Orb of="qa" size={10} /></span><span {...stylex.props(s.orbStack)}><Orb of="director" size={10} /></span></span>}
      onClick={onToggle}>{c.button}{count > 0 && <span {...stylex.props(s.count)}>{count}</span>}</ActionButton>
  </div>
}

/** Nhóm thẻ gọn ở cột trái, mở bằng nút “Đề xuất N”. */
export function SuggestionDock({ scope, onSeeAll }: { scope: string; onSeeAll: () => void }) {
  const { list, accept, dismiss, busy, notice, online } = useSuggestions(scope)
  const items = list.data?.suggestions.slice(0, 3) ?? []
  const total = list.data?.total ?? 0
  return <section aria-label={c.region} {...stylex.props(s.group)}>
    <h2 {...stylex.props(s.groupHead)}><span>{c.group}</span><span>{total}</span></h2>
    {list.initialLoading ? <p role="status" {...stylex.props(s.state)}>{c.loading}</p>
      : list.error && !list.data ? <div role="alert" {...stylex.props(s.state)}>{c.error}<ActionButton variant="quiet" size="sm" type="button" className={stylex.props(s.stateAction).className} onClick={() => void list.refresh()}>{c.retry}</ActionButton></div>
      : items.length === 0 ? <p {...stylex.props(s.state)}>{c.empty}</p>
      : <>
        {!online && <p role="status" {...stylex.props(s.state)}>{c.offline}</p>}
        {items.map((item) => <SuggestionCard key={item.id} suggestion={item} compact busy={busy === item.id} offline={!online}
          notice={notice?.id === item.id ? notice.text : undefined} onAccept={() => void accept(item)} onDismiss={() => void dismiss(item)} />)}
        {total > items.length && <button type="button" {...stylex.props(s.seeAll)} onClick={onSeeAll}>{c.seeAll(total)}</button>}
      </>}
  </section>
}

/** Mục “Đề xuất · 3 trong 5” của Bản tin; không có đề xuất thì không chiếm chỗ. */
export function RecapSuggestions() {
  const { list, accept, dismiss, busy, notice, online } = useSuggestions('all')
  const items = list.data?.suggestions.slice(0, 3) ?? []
  const total = list.data?.total ?? 0
  if (list.initialLoading) return <section aria-label={c.region} {...stylex.props(s.recapSection)}><p role="status" {...stylex.props(s.state)}>{c.loading}</p></section>
  if (list.error && !list.data) return <section aria-label={c.region} {...stylex.props(s.recapSection)}><div role="alert" {...stylex.props(s.state)}>{c.error}<ActionButton variant="quiet" size="sm" type="button" className={stylex.props(s.stateAction).className} onClick={() => void list.refresh()}>{c.retry}</ActionButton></div></section>
  if (items.length === 0) return null
  return <section aria-label={c.region} {...stylex.props(s.recapSection)}>
    <span {...stylex.props(s.recapGroup)}>{c.recapGroup(items.length, total)}</span>
    {!online && <p role="status" {...stylex.props(s.state)}>{c.offline}</p>}
    <div {...stylex.props(s.recapList)}>
      {items.map((item) => <SuggestionCard key={item.id} suggestion={item} busy={busy === item.id} offline={!online}
        notice={notice?.id === item.id ? notice.text : undefined} onAccept={() => void accept(item)} onDismiss={() => void dismiss(item)} onNever={() => void dismiss(item, true)} />)}
    </div>
  </section>
}
