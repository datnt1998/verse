// /diag — bảng “Sức khoẻ truyện” (CmdResults.dc.html view=diagLoading → diag, GAP-28).
// Chạy GET …/diag (máy chủ đồng thời hiện việc `diag` trong dock nên đóng bảng vẫn chạy ngầm);
// `mode="last"` mở lại báo cáo gần nhất (nút “Xem báo cáo” ở dock) mà không phân tích lại.
import { useEffect, useMemo, useRef, useState } from 'react'
import { ApiError, downloadDiagExport, getDiag, getLastDiag, sendDiagAction } from '../../api/client'
import type { DiagReport } from '../../api/types'
import { setData, useAction, useResource } from '../../data'
import { enumLabel } from '../../i18n/enums'
import { vi } from '../../i18n/vi'
import { AgentOrb } from '@noverse/ui'
import { Kbd } from '@noverse/ui'
import { Dialog } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import './cmdresults.css'
import {
  CATEGORY_LABEL, CONFIDENCE_LABEL, SEVERITY_META, diagTiles, filterFindings, formatInt, severityCounts,
  type SeverityFilter,
} from './logic'
import { OutlineBtn, errorText } from './parts'

const FILTERS: { key: SeverityFilter; label: string; dot: string }[] = [
  { key: 'all', label: 'Tất cả', dot: '#A0A0AB' },
  { key: 'critical', label: SEVERITY_META.critical.label, dot: SEVERITY_META.critical.color },
  { key: 'warning', label: SEVERITY_META.warning.label, dot: SEVERITY_META.warning.color },
  { key: 'info', label: SEVERITY_META.info.label, dot: SEVERITY_META.info.color },
]

const pad = (n: number) => String(n).padStart(2, '0')

export function DiagPanel({ project, story, mode = 'run', onClose }: {
  project: string; story: string; mode?: 'run' | 'last'; onClose: () => void
}) {
  const resourceKey = ['story', project, story, 'diag'] as const
  const firstRun = useRef(mode === 'run')
  const resource = useResource<DiagReport>(resourceKey, async ({ signal }) => {
    if (firstRun.current) {
      const next = await getDiag(project, story, signal)
      firstRun.current = false
      return next
    }
    try { return await getLastDiag(project, story, signal) } catch (reason) {
      if (reason instanceof ApiError && reason.status === 404) return getDiag(project, story, signal)
      throw reason
    }
  }, { staleTime: 0 })
  const report = resource.data
  const error = resource.error ? errorText(resource.error, 'Không chạy được chẩn đoán') : null
  const loading = resource.initialLoading
  const [filter, setFilter] = useState<SeverityFilter>('all')
  const [open, setOpen] = useState<Set<string>>(() => new Set())
  const [sent, setSent] = useState<Set<string>>(() => new Set())
  const [sending, setSending] = useState<string | null>(null)
  const rerun = useAction(async () => {
    const next = await getDiag(project, story)
    setData(resourceKey, next)
  }, { error: 'Không chạy được chẩn đoán' })
  useEffect(() => {
    if (!report) return
    setSent(new Set(report.actions.filter((action) => action.sent).map((action) => action.fingerprint)))
    const first = filterFindings(report.findings, 'all')[0]
    setOpen(new Set(first ? [first.fingerprint] : []))
  }, [report])

  const counts = useMemo(() => severityCounts(report?.findings ?? []), [report])
  const findings = useMemo(() => filterFindings(report?.findings ?? [], filter), [report, filter])
  const tiles = useMemo(() => (report ? diagTiles(report.stats, report.findings, (phase) => enumLabel('phase', phase)) : []), [report])

  const toggle = (fingerprint: string) => setOpen((previous) => {
    const next = new Set(previous)
    if (next.has(fingerprint)) next.delete(fingerprint); else next.add(fingerprint)
    return next
  })

  const sendAction = useAction(async (fingerprint: string) => {
    if (sent.has(fingerprint)) return
    setSending(fingerprint)
    try {
      await sendDiagAction(project, story, fingerprint)
      setSent((previous) => new Set(previous).add(fingerprint))
    } catch (reason) {
      if (reason instanceof ApiError && reason.status === 404) throw new Error('Đề xuất này đã cũ — hãy phân tích lại.')
      throw reason
    } finally { setSending(null) }
  }, { success: 'Đã gửi cho trợ lý', error: 'Không gửi được cho trợ lý', invalidate: [['story', project, story, 'snapshot']] })
  const exportAction = useAction(async () => {
    await downloadDiagExport(project, story)
  }, { success: 'Đã tải chẩn đoán ẩn danh', error: 'Không xuất được chẩn đoán' })

  const generated = report ? new Date(report.generatedAt) : null
  const generatedAt = generated && !Number.isNaN(generated.getTime()) ? `${pad(generated.getHours())}:${pad(generated.getMinutes())}` : '—'

  return (
    <Dialog onClose={onClose} width={1000} style={{ height: 680, maxHeight: '100%', overflow: 'hidden' }}>
      <div className="cr-dlg-in" style={{ display: 'flex', flexDirection: 'column', height: '100%', minHeight: 0 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '16px 18px', borderBottom: '1px solid rgba(255,255,255,0.06)' }}>
          <Icon name="activity" size={18} style={{ color: '#C9A6F5' }} />
          <h2 style={{ margin: 0, flex: 1, fontSize: 16, fontWeight: 600 }}>Sức khoẻ truyện</h2>
          <span className="mono" style={{ fontSize: 12, color: '#7E7E8A' }}>/diag</span>
          <Kbd>Esc</Kbd>
        </div>
        {(resource.refreshing || rerun.pending) && report && <div className="cr-bar cr-indet" aria-label="Đang cập nhật" style={{ height: 2, flexShrink: 0 }}><i /></div>}
        {error && report && <div role="alert" style={{ padding: '8px 18px', color: '#F6CB8C', fontSize: 12 }}>Không tải lại được · <button onClick={() => void resource.refresh()}>Thử lại</button></div>}

        {loading ? (
          <div role="status" aria-live="polite" style={{ flex: '1 1 auto', minHeight: 0, padding: 22, display: 'flex', flexDirection: 'column', gap: 18 }}>
            <div style={{ display: 'flex', gap: 16, alignItems: 'center' }}>
              <span aria-hidden="true" style={{ position: 'relative', width: 54, height: 54, flex: '0 0 auto' }}>
                <span className="cr-orbit" style={{ position: 'absolute', inset: 0 }}>
                  <AgentOrb agent="architect" size={16} style={{ position: 'absolute', left: 0, top: 18 }} />
                  <AgentOrb agent="writer" size={16} style={{ position: 'absolute', right: 0, top: 18 }} />
                  <AgentOrb agent="editor" size={16} style={{ position: 'absolute', left: 19, top: 0 }} />
                  <AgentOrb agent="arbiter" size={16} style={{ position: 'absolute', left: 19, bottom: 0 }} />
                </span>
              </span>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 2 }}>
                <span className="cr-shim" style={{ fontSize: 15, fontWeight: 600 }}>Đang phân tích và tạo báo cáo chẩn đoán…</span>
                <span style={{ fontSize: 12.5, color: '#A0A0AB' }}>Đang đọc chương, đại cương và nhật ký để xem quy trình, chất lượng, đại cương và bối cảnh.</span>
              </div>
            </div>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gap: 10 }}>
              {[0, 1, 2, 3].map((i) => <span key={i} className="cr-skel" style={{ height: 74 }} />)}
            </div>
            <span className="cr-skel" style={{ height: 120 }} />
            <span className="cr-skel" style={{ height: 120 }} />
            <div style={{ display: 'flex', gap: 8, alignItems: 'center', fontSize: 12.5, color: '#8E8E99' }}>
              <Icon name="info-circle" size={14} />Có thể đóng bảng — phân tích vẫn chạy ngầm và báo ở góc dưới phải khi xong.
            </div>
          </div>
        ) : !report ? (
          <div role="alert" style={{ flex: '1 1 auto', display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: 12, padding: 24 }}>
            <Icon name="warning-2" size={22} style={{ color: '#FF9FA8' }} />
            <span style={{ fontSize: 13.5, color: '#D6D6DC', textAlign: 'center' }}>{error ?? 'Chưa có báo cáo'}</span>
            <OutlineBtn icon="refresh-2" onClick={() => void resource.refresh()}>Phân tích lại</OutlineBtn>
          </div>
        ) : (
          <>
            <div className="scroll" style={{ flex: '1 1 auto', minHeight: 0, overflowY: 'auto', padding: '18px 20px', display: 'flex', flexDirection: 'column', gap: 18 }}>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gap: 10 }}>
                {tiles.map((tile, i) => (
                  <div key={tile.key} className="card cr-rise" style={{
                    borderRadius: 12, padding: '12px 14px', display: 'flex', flexDirection: 'column', gap: 2, animationDelay: `${(0.03 * i).toFixed(2)}s`,
                  }}>
                    <span className="lbl" style={{ fontSize: 10.5 }}>{tile.key}</span>
                    <span className="num" style={{ fontSize: 20, fontWeight: 600, letterSpacing: '-0.01em' }}>{tile.value}</span>
                    <span style={{ fontSize: 11.5, color: tile.warn ? '#F6CB8C' : '#7E7E8A' }}>{tile.sub}</span>
                  </div>
                ))}
              </div>

              <div style={{ display: 'flex', alignItems: 'center', gap: 10, flexWrap: 'wrap' }}>
                <h3 style={{ margin: 0, fontSize: 14.5, fontWeight: 600, flex: '1 1 auto' }}>Phát hiện</h3>
                <div role="tablist" aria-label="Lọc theo mức" style={{ display: 'flex', gap: 4 }}>
                  {FILTERS.map((f) => {
                    const sel = filter === f.key
                    return (
                      <button key={f.key} type="button" role="tab" aria-selected={sel} onClick={() => setFilter(f.key)} style={{
                        height: 28, padding: '0 10px', borderRadius: 999, border: `1px solid ${sel ? 'rgba(255,255,255,0.16)' : 'rgba(255,255,255,0.08)'}`,
                        background: sel ? '#26262A' : 'transparent', color: sel ? '#EDEDEF' : '#A0A0AB', fontSize: 12,
                        display: 'flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap',
                      }}>
                        <span aria-hidden="true" style={{ width: 7, height: 7, borderRadius: '50%', background: f.dot }} />
                        {f.label}<span className="num" style={{ color: '#7E7E8A' }}>{counts[f.key]}</span>
                      </button>
                    )
                  })}
                </div>
              </div>

              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                {findings.length === 0 && (
                  <div style={{ padding: '14px 16px', borderRadius: 12, border: '1px dashed rgba(255,255,255,0.1)', fontSize: 13, color: '#A0A0AB', display: 'flex', gap: 8, alignItems: 'center' }}>
                    <Icon name="tick-circle" size={16} style={{ color: '#A6EDC1' }} />
                    {filter === 'all' ? 'Không phát hiện vấn đề nào.' : 'Không có phát hiện ở mức này.'}
                  </div>
                )}
                {findings.map((finding, j) => {
                  const isOpen = open.has(finding.fingerprint)
                  const sev = SEVERITY_META[finding.severity]
                  return (
                    <div key={finding.fingerprint} className="card cr-rise" style={{ borderRadius: 12, overflow: 'hidden', animationDelay: `${(0.05 * j + 0.1).toFixed(2)}s` }}>
                      <button className="cr-crow" type="button" onClick={() => toggle(finding.fingerprint)} aria-expanded={isOpen} style={{
                        width: '100%', display: 'flex', alignItems: 'center', gap: 12, padding: '12px 14px', border: 'none',
                        background: 'transparent', color: '#EDEDEF', textAlign: 'left',
                      }}>
                        <span aria-label={sev.label} style={{ width: 8, height: 8, borderRadius: '50%', background: sev.color, flex: '0 0 auto', boxShadow: `0 0 0 4px ${sev.halo}` }} />
                        <span style={{ flex: '1 1 auto', minWidth: 0, fontSize: 13.5, fontWeight: 500 }}>{finding.title}</span>
                        <span style={{
                          height: 22, padding: '0 8px', borderRadius: 6, background: 'rgba(255,255,255,0.05)', color: '#C9C9D1',
                          fontSize: 11.5, display: 'flex', alignItems: 'center', whiteSpace: 'nowrap',
                        }}>{CATEGORY_LABEL[finding.category] ?? finding.category}</span>
                        <span style={{ fontSize: 11.5, color: '#7E7E8A', whiteSpace: 'nowrap', width: 92, textAlign: 'right' }}>{CONFIDENCE_LABEL[finding.confidence] ?? finding.confidence}</span>
                        <Icon name={isOpen ? 'arrow-down-2' : 'arrow-right-2'} size={15} style={{ color: '#7E7E8A' }} />
                      </button>
                      {isOpen && (
                        <div style={{ padding: '0 14px 14px 34px', display: 'grid', gridTemplateColumns: 'minmax(0, 1fr) minmax(0, 1fr)', gap: 12 }}>
                          <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                            <span className="lbl" style={{ fontSize: 10.5 }}>Bằng chứng</span>
                            <span style={{ fontSize: 12.5, color: '#C9C9D1', whiteSpace: 'pre-wrap' }}>{finding.evidence || '—'}</span>
                          </div>
                          <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                            <span className="lbl" style={{ fontSize: 10.5 }}>Nên làm</span>
                            <span style={{ fontSize: 12.5, color: '#C9C9D1', whiteSpace: 'pre-wrap' }}>{finding.suggestion || '—'}</span>
                          </div>
                          {finding.technical && <details style={{ gridColumn: '1 / -1', minWidth: 0 }}>
                            <summary>{vi.adv.title}</summary>
                            <pre className="mono" style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere' }}>{finding.technical}</pre>
                          </details>}
                        </div>
                      )}
                    </div>
                  )
                })}
              </div>

              {report.actions.length > 0 && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                  <h3 style={{ margin: 0, fontSize: 14.5, fontWeight: 600 }}>
                    Hành động đề xuất <span className="num" style={{ color: '#7E7E8A', fontWeight: 400 }}>({report.actions.length})</span>
                  </h3>
                  {report.actions.map((action) => {
                    const done = sent.has(action.fingerprint)
                    const busy = sending === action.fingerprint && sendAction.pending
                    return (
                      <div key={action.fingerprint} style={{
                        display: 'flex', alignItems: 'center', gap: 12, padding: '10px 12px', borderRadius: 12,
                        background: 'rgba(201,166,245,0.05)', border: '1px solid rgba(201,166,245,0.16)',
                      }}>
                        <Icon name="magic-star" size={16} style={{ color: '#C9A6F5' }} />
                        <span style={{ flex: '1 1 auto', minWidth: 0, fontSize: 13 }}>{action.summary}</span>
                        <button className="cr-tbtn" type="button" onClick={() => void sendAction.run(action.fingerprint)} disabled={done || sendAction.busy} aria-busy={busy} style={{
                          height: 30, padding: '0 12px', borderRadius: 8, border: '1px solid rgba(255,255,255,0.1)',
                          background: done ? 'transparent' : '#EDEDEF', color: done ? '#A6EDC1' : '#0B0B0D', fontSize: 12, fontWeight: 500,
                          display: 'flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap',
                        }}>
                          {busy ? <span className="cr-spinner" style={{ width: 12, height: 12, borderTopColor: '#0B0B0D' }} /> : <Icon name={done ? 'tick-circle' : 'arrow-up'} size={14} />}
                          {done ? 'Đã gửi' : 'Gửi cho trợ lý'}
                        </button>
                      </div>
                    )
                  })}
                </div>
              )}
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 10, padding: '12px 18px', borderTop: '1px solid rgba(255,255,255,0.06)' }}>
              <span style={{ fontSize: 12, color: '#7E7E8A' }}>Phân tích lúc {generatedAt} · dựa trên {formatInt(report.stats.completedChapters)} chương đã xong</span>
              <span style={{ flex: 1 }} />
              <OutlineBtn icon="document-download" onClick={() => void exportAction.run()} disabled={exportAction.busy}>Xuất chẩn đoán ẩn danh</OutlineBtn>
              <OutlineBtn icon="refresh-2" onClick={() => void rerun.run()} disabled={rerun.busy}>Phân tích lại</OutlineBtn>
            </div>
          </>
        )}
      </div>
    </Dialog>
  )
}
