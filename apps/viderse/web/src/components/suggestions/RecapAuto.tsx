// Một dòng “Máy đã tự làm” trong Bản tin (.vv-recap__line): chữ, Vì sao?, thẻ Tự làm, Xem
// và Hoàn tác khi việc đó đảo ngược được (`auto[].undo`).
import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Icon, toast } from '@noverse/ui'
import { ApiError } from '../../api/request'
import { undoOp } from '../../api/suggestions'
import type { RecapData } from '../../api/workbench'
import { t } from '../../i18n/vi'
import { navigate } from '../../router'
import { ActionButton } from '../ActionButton'
import { ReasonPopover } from '../reason/ReasonPopover'
import { suggestStyles as s } from './suggestions.stylex'
import { refreshSuggestions } from './useSuggestions'

const c = t.learn.suggest

export function RecapAutoLine({ item, offline = false }: { item: NonNullable<RecapData['auto']>[number]; offline?: boolean }) {
  const [why, setWhy] = useState(false)
  const [busy, setBusy] = useState(false)
  const [done, setDone] = useState(false)
  const undo = item.undo
  const revert = async () => {
    if (!undo) return
    setBusy(true)
    try { await undoOp(undo); setDone(true); refreshSuggestions() } catch (error) {
      toast.err(error instanceof ApiError && error.status !== 0 ? error.message : c.undoFailed)
    } finally { setBusy(false) }
  }
  return <>
    <div {...stylex.props(s.autoLine)}>
      <span {...stylex.props(s.autoText)}>{item.text}</span>
      {item.reason && <button type="button" aria-expanded={why} {...stylex.props(s.why, why && s.whyOpen)} onClick={() => setWhy(!why)}><Icon name="info" size={12} />{c.why}</button>}
      <span {...stylex.props(s.autoTag)}>{t.learn.suggest.autoTag}</span>
      {item.href && <ActionButton variant="quiet" size="sm" type="button" onClick={() => navigate(item.href.replace(/^#/, ''))}>{t.learn.suggest.view}</ActionButton>}
      {undo && !done && <ActionButton variant="quiet" size="sm" type="button" disabled={busy || offline} disabledReason={offline ? c.offlineAction : undefined} onClick={() => void revert()}>{c.undo}</ActionButton>}
    </div>
    {why && item.reason && <ReasonPopover reason={item.reason} label={c.why} />}
  </>
}
