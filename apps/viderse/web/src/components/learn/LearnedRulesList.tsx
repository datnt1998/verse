// Quy tắc đã học (Kênh › Cài đặt) và Ghi chú đã học (Thư viện › Prompt ảnh): công tắc, nguồn,
// Xem ví dụ, Xoá (có xác nhận). Trạng thái tải / trống / lỗi / mất kết nối đều có.
import { useState } from 'react'
import * as stylex from '@stylexjs/stylex'
import { Icon, confirm, toast } from '@noverse/ui'
import { deleteLearnedRule, setLearnedRuleEnabled, type LearnedRule, type RuleScope } from '../../api/learnedRules'
import { useOnline } from '../../data/status'
import { t } from '../../i18n/vi'
import { ActionButton } from '../ActionButton'
import { RuleExamples } from './RuleExamples'
import { learnStyles as s } from './learn.stylex'
import { refreshLearned, useLearnedRules } from './learnedRules'

const c = t.learn.rules

function RuleRow({ rule, offline, onChanged }: { rule: LearnedRule; offline: boolean; onChanged: () => void }) {
  const [open, setOpen] = useState(false)
  const [busy, setBusy] = useState(false)
  const examples = rule.learnedFrom.examples
  const toggle = async () => {
    setBusy(true)
    try { await setLearnedRuleEnabled(rule.id, !rule.enabled); onChanged() } catch { toast.err(c.toggleFailed) } finally { setBusy(false) }
  }
  const remove = async () => {
    const ok = await confirm({ title: c.removeTitle(rule.text), body: c.removeBody, tone: 'danger', confirmLabel: c.removeOk, cancelLabel: t.learn.suggest.confirm.cancel, onConfirm: () => undefined })
    if (!ok) return
    setBusy(true)
    try { await deleteLearnedRule(rule.id); onChanged() } catch { toast.err(c.removeFailed) } finally { setBusy(false) }
  }
  return <li {...stylex.props(s.rule)}>
    <button type="button" role="switch" aria-checked={rule.enabled} aria-label={rule.text} disabled={busy || offline} {...stylex.props(s.ruleSwitch, rule.enabled && s.ruleSwitchOn)} onClick={() => void toggle()}>
      <span {...stylex.props(s.knob, rule.enabled && s.knobOn)} />
    </button>
    <span {...stylex.props(s.ruleText)}>{rule.text}</span>
    <span {...stylex.props(s.ruleActions)}>
      <ActionButton variant="quiet" size="sm" type="button" disabled={busy || offline} aria-label={c.remove(rule.text)} onClick={() => void remove()}><Icon name="trash" size={14} /></ActionButton>
    </span>
    <span {...stylex.props(s.ruleSrc)}>{rule.source}{!rule.enabled && ` · ${c.off}`}
      {examples.length > 0 && <> · <button type="button" aria-expanded={open} {...stylex.props(s.linkBtn)} onClick={() => setOpen(!open)}>{open ? c.hideExample : c.viewExample}</button></>}
    </span>
    {open && <div {...stylex.props(s.ruleExamples)}><RuleExamples examples={examples} /></div>}
  </li>
}

export function LearnedRulesList({ scope, title = c.title, help = c.help, empty = c.empty, errorText = c.error }: {
  scope: RuleScope
  title?: string
  help?: string
  empty?: string
  errorText?: string
}) {
  const online = useOnline()
  const { data, initialLoading, error, refresh } = useLearnedRules(scope)
  return <section aria-label={title} {...stylex.props(s.rules)}>
    <div {...stylex.props(s.rulesHead)}>
      <span {...stylex.props(s.rulesTitle)}>{title}</span>
      {data && <span {...stylex.props(s.counter)}>{c.counter(data.rules.length, data.limit)}</span>}
      <span {...stylex.props(s.rulesHelp)}>{help}</span>
    </div>
    {initialLoading ? <p role="status" {...stylex.props(s.state)}>{c.loading}</p>
      : error && !data ? <div role="alert" {...stylex.props(s.state)}>{errorText}<ActionButton variant="quiet" size="sm" type="button" className={stylex.props(s.stateAction).className} onClick={() => void refresh()}>{c.retry}</ActionButton></div>
      : data && data.rules.length === 0 ? <p {...stylex.props(s.state)}>{empty}</p>
      : data && <>
        {!online && <p role="status" {...stylex.props(s.state)}>{c.offline}</p>}
        <ul style={{ margin: 0, padding: 0, listStyle: 'none' }}>{data.rules.map((rule) => <RuleRow key={rule.id} rule={rule} offline={!online} onChanged={refreshLearned} />)}</ul>
      </>}
  </section>
}
