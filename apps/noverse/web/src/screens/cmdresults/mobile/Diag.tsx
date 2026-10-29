import { useEffect, useRef, useState } from 'react'
import { ApiError, getDiag, getLastDiag, sendDiagAction } from '../../../api/client'
import { keys, setData, useAction, useResource } from '../../../data'
import { enumLabel } from '../../../i18n/enums'
import { vi } from '../../../i18n/vi'
import { Icon } from '@noverse/ui'
import { CATEGORY_LABEL, CONFIDENCE_LABEL, filterFindings, severityCounts, type SeverityFilter } from '../logic'
import { MOBILE_SEVERITY_META as SEVERITY_META, mobileDiagTiles } from './logic'
import { Button, Failure, Label, Lamp, Loading, useCmdToast } from './shared'
import { PanelRefreshBar } from '../../../ui/PanelRefreshBar'

/** “hôm nay 14:32” cho cùng ngày, “23/10 14:32” cho ngày khác. */
function lastRunLabel(iso: string, now = new Date()): string {
  const date = new Date(iso)
  if (Number.isNaN(date.getTime())) return '—'
  const time = date.toLocaleTimeString('vi-VN', { hour: '2-digit', minute: '2-digit', hour12: false })
  if (date.toDateString() === now.toDateString()) return `hôm nay ${time}`
  return `${String(date.getDate()).padStart(2, '0')}/${String(date.getMonth() + 1).padStart(2, '0')} ${time}`
}

export function Diag({ project, story, existing, onWrite }: { project: string; story: string; existing: boolean; onWrite: () => void }) {
  const key = ['story', project, story, 'diag'] as const
  const firstRun = useRef(!existing)
  const resource = useResource(key, async ({ signal }) => {
    if (firstRun.current) {
      const next = await getDiag(project, story, signal)
      firstRun.current = false
      return next
    }
    try { return await getLastDiag(project, story, signal) }
    catch (error) { if (error instanceof ApiError && error.status === 404) return getDiag(project, story, signal); throw error }
  }, { staleTime: 0 })
  const report = resource.data
  const [filter, setFilter] = useState<SeverityFilter>('all')
  const [open, setOpen] = useState<Set<string>>(new Set())
  const [sent, setSent] = useState<Set<string>>(new Set())
  const [sending, setSending] = useState('')
  const [minimumLoading, setMinimumLoading] = useState(false)
  const skeletonTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  useEffect(() => () => clearTimeout(skeletonTimer.current), [])
  const notify = useCmdToast()
  useEffect(() => {
    const first = filterFindings(report?.findings ?? [], 'all')[0]
    setOpen(new Set(first ? [first.fingerprint] : []))
    setSent(new Set(report?.actions.filter((action) => action.sent).map((action) => action.fingerprint)))
  }, [report])
  const rerun = useAction(async () => {
    setMinimumLoading(true)
    clearTimeout(skeletonTimer.current)
    skeletonTimer.current = setTimeout(() => setMinimumLoading(false), 1700)
    const next = await getDiag(project, story)
    setData(key, next)
  }, { error: false })
  const send = useAction(async (fingerprint: string) => {
    setSending(fingerprint)
    try { await sendDiagAction(project, story, fingerprint); setSent((previous) => new Set(previous).add(fingerprint)); notify('Đã gửi vào hàng chờ của trợ lý', onWrite) }
    finally { setSending('') }
  }, { error: false, invalidate: [keys.story(project, story, 'snapshot')] })
  const counts = severityCounts(report?.findings ?? [])
  const findings = filterFindings(report?.findings ?? [], filter)
  return <div className="mc-scroll mc-diag">
    <PanelRefreshBar resourceKeys={[resource.key]} />
    <Failure error={resource.error || rerun.error} onRetry={() => void resource.refresh()} />
    {(!report && !resource.error) || rerun.busy || minimumLoading ? <Loading text="Đang đọc các chương và nhật ký…" /> : report && <>
      <div className="mc-stats">{mobileDiagTiles(report.stats, report.findings, (phase) => enumLabel('phase', phase)).map((tile, index) => <div className="mc-stat mc-rise" key={tile.key} style={{ animationDelay: `${(index * 0.03).toFixed(2)}s` }}><span>{tile.key}</span><strong>{tile.value}</strong><small className={tile.warn ? 'mc-warn-text' : ''}>{tile.sub}</small></div>)}</div>
      <Label>PHÁT HIỆN</Label>
      <div className="mc-filters" role="tablist" aria-label="Lọc theo mức độ">{(['all', ...(counts.critical ? ['critical'] as const : []), 'warning', 'info'] as SeverityFilter[]).map((severity) => <button key={severity} type="button" role="tab" className={`mc-chip ${filter === severity ? 'mc-selected' : ''}`} aria-selected={filter === severity} onClick={() => setFilter(severity)}><i aria-hidden="true" style={{ background: severity === 'all' ? '#C9C9D1' : SEVERITY_META[severity].color }} />{severity === 'all' ? 'Tất cả' : SEVERITY_META[severity].label} <span className="mc-num">{counts[severity]}</span></button>)}</div>
      <div className="mc-findings">
        {findings.length === 0 && <p className="mc-note">Không có phát hiện ở mức này.</p>}
        {findings.map((finding) => { const expanded = open.has(finding.fingerprint); const meta = SEVERITY_META[finding.severity]; return <section className="mc-finding mc-rise" key={finding.fingerprint}>
          <button type="button" onClick={() => setOpen((previous) => { const next = new Set(previous); if (next.has(finding.fingerprint)) next.delete(finding.fingerprint); else next.add(finding.fingerprint); return next })} aria-expanded={expanded}>
            <span className="mc-severity" aria-hidden="true" style={{ background: meta.halo }}><i style={{ background: meta.color }} /></span>
            <span><strong>{finding.title}</strong><small>{meta.label} · {CATEGORY_LABEL[finding.category]} · {CONFIDENCE_LABEL[finding.confidence]}</small></span>
            <Icon name="arrow-down-2" size={16} className={`mc-chev ${expanded ? 'mc-up' : ''}`} />
          </button>
          {expanded && <div className="mc-finding-body">
            <p>{finding.evidence}</p><p className="mc-suggestion"><Lamp />{finding.suggestion}</p>
            {finding.technical && <details><summary>{vi.adv.title}</summary><pre className="mono" style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{finding.technical}</pre></details>}
          </div>}
        </section> })}
      </div>
      {report.actions.length > 0 && <><Label>VIỆC NÊN LÀM</Label><div className="mc-group">{report.actions.map((action) => { const isSent = sent.has(action.fingerprint); return <div className="mc-row mc-action-row" key={action.fingerprint}><span>{action.summary}</span><button type="button" className={`mc-small-button ${isSent ? 'mc-sent' : ''}`} aria-label={isSent ? 'Đã gửi cho trợ lý' : 'Gửi cho trợ lý'} disabled={isSent || send.busy} onClick={() => void send.run(action.fingerprint)}><Icon name={isSent ? 'tick-circle' : 'arrow-up'} size={14} />{isSent ? 'Đã gửi' : sending === action.fingerprint ? 'Đang gửi…' : 'Gửi'}</button></div> })}</div></>}
      <Failure error={send.error} />
      <Button className="mc-rerun" disabled={rerun.busy} onClick={() => void rerun.run()}><Icon name="refresh-2" size={18} />Phân tích lại</Button>
      <p className="mc-last-run">Lần cuối: {lastRunLabel(report.generatedAt)} · chỉ đọc, không thay đổi gì trong truyện</p>
    </>}
  </div>
}
