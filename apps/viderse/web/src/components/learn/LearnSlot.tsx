// Banner học dưới trường vừa sửa (Main `agent=learn`) và dưới ảnh vừa góp ý vẽ lại
// (Review `state=learn`). Một đề xuất `learn_rule` từ server: Lưu quy tắc · Không · Xem N lần sửa.
// Lưu xong banner đổi thành dòng xanh có Hoàn tác; slot giữ đề xuất đã thấy cho tới khi người dùng
// bấm Không / Hoàn tác hoặc server gửi đề xuất mới.
import { useEffect, useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Icon } from '@noverse/ui'
import { acceptSuggestion, dismissSuggestion, undoOp, type AcceptResult, type Suggestion } from '../../api/suggestions'
import { ApiError } from '../../api/request'
import { useOnline } from '../../data/status'
import { t } from '../../i18n/vi'
import { ActionButton } from '../ActionButton'
import { Orb } from '../Orb'
import { RuleExamples } from './RuleExamples'
import { learnStyles as s } from './learn.stylex'
import { learnExamples, refreshLearned } from './learnedRules'

const c = t.learn.learn

export function LearnSlot({ learn }: { learn: Suggestion | null | undefined }) {
  const [shown, setShown] = useState<Suggestion | null>(learn ?? null)
  const [saved, setSaved] = useState<AcceptResult | null>(null)
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const online = useOnline()
  useEffect(() => {
    if (learn && learn.id !== shown?.id) { setShown(learn); setSaved(null); setOpen(false); setError('') }
  }, [learn, shown?.id])
  if (!shown) return null
  const artist = shown.agent === 'artist'
  const examples = learnExamples(shown)
  const fail = (cause: unknown) => setError(cause instanceof ApiError && cause.status !== 0 ? cause.message : cause instanceof ApiError ? c.offline : c.saveFailed)
  const save = async () => {
    setBusy(true); setError('')
    try {
      const outcome = await acceptSuggestion(shown.id, true)
      if (outcome.kind === 'done') { setSaved(outcome.result); refreshLearned() } else if (outcome.kind === 'gone') { setShown(null); refreshLearned() }
    } catch (cause) { fail(cause) } finally { setBusy(false) }
  }
  const decline = async () => {
    setBusy(true); setError('')
    try { await dismissSuggestion(shown.id); setShown(null); refreshLearned() } catch (cause) { fail(cause) } finally { setBusy(false) }
  }
  const undo = async () => {
    const op = saved?.undo
    if (!op) { setShown(null); setSaved(null); return }
    setBusy(true); setError('')
    try { await undoOp(op); setSaved(null); refreshLearned() } catch (cause) { fail(cause) } finally { setBusy(false) }
  }
  if (saved) {
    const promptName = typeof shown.primary.params.promptName === 'string' ? shown.primary.params.promptName : ''
    return <div role="status" {...stylex.props(s.saved)}>
      <span {...stylex.props(s.savedIcon)}><Icon name="tick-b" size={15} /></span>
      <span {...stylex.props(s.savedText)}>{artist ? c.savedPrompt(promptName) : c.savedChannel}</span>
      {saved.undo && <ActionButton variant="quiet" size="sm" type="button" disabled={busy || !online} onClick={() => void undo()}>{c.undo}</ActionButton>}
    </div>
  }
  return <div role="status" {...stylex.props(s.learn, artist && s.learnArtist)}>
    <span {...stylex.props(s.orb)}><Orb of={shown.agent} size={16} /></span>
    <span {...stylex.props(s.text)}>{shown.banner ?? shown.title}</span>
    <div {...stylex.props(s.actions)}>
      <ActionButton variant="primary" size="sm" type="button" disabled={busy || !online} disabledReason={!online ? c.offline : undefined} onClick={() => void save()}>{shown.primary.label}</ActionButton>
      <ActionButton variant="quiet" size="sm" type="button" disabled={busy || !online} onClick={() => void decline()}>{c.no}</ActionButton>
      {examples.length > 0 && <ActionButton variant="quiet" size="sm" type="button" aria-expanded={open} onClick={() => setOpen(!open)}>{open ? c.hideExamples : c.examples(examples.length)}</ActionButton>}
    </div>
    {open && <RuleExamples examples={examples} />}
    {error && <div role="alert" {...stylex.props(s.notice)}>{error}</div>}
  </div>
}
