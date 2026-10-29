// Toast của dock — 14-jobs-dock.md §1.4: Xong (mint) · Cần bạn (cam) · Lỗi (đỏ hồng).
// Đếm ngược tự đóng = vòng mảnh r=12 quanh nút ×; rê chuột vào chồng toast thì dừng.
import type { Job } from '../../api/types'
import { AGENT_PALETTE, agentAura, agentOrbGlow } from '../../ui/agentPalette'
import { Icon } from '@noverse/ui'
import { TickMark } from './JobRing'
import { CtaButton, resultCtas, type Cta } from './JobRow'
import { DONE_COLOR, FAIL_COLOR, TOAST_MS, WAIT_COLOR } from './logic'
import {
  exportLook, exportedHref, isAudioExportJob, jobError, type ExportActionId,
} from './exportFlow'
import {
  dismissToast, dispatchDock, downloadExportManual, openExportFile, openHref, retryExport, retryJob,
  shareExportJob, type JobToast as ToastModel,
} from './store'

const CIRCUMFERENCE = 75.4 // 2π·12

type Look = { title: string; sub: string; note?: string; noteColor?: string; bar?: number | null; icon?: string; tint: string; iconBg: string; ring: string; ctas: Cta[] }

/** §AE — việc ghép & xuất audio có nhánh toast riêng (tải tay · tự tải · bị chặn · lỗi có chương). */
function exportCtas(job: Job, close: () => void): Record<ExportActionId, Cta> {
  const download = {
    icon: 'document-download', label: 'Tải xuống', primary: true,
    run: async () => { await downloadExportManual(job) },
  }
  return {
    openFile: { icon: 'play', label: 'Mở tệp', run: () => { openExportFile(job); close() } },
    openExport: { icon: 'folder-open', label: 'Xem bản xuất', run: () => { close(); openHref(exportedHref(job)) } },
    download,
    share: { icon: 'link-2', label: 'Chia sẻ', run: () => shareExportJob(job) },
    retry: { icon: 'refresh-2', label: 'Thử lại', primary: true, run: async () => { await retryExport(job); close() } },
    retryNoMusic: { icon: 'music', label: 'Ghép không nhạc', run: async () => { await retryExport(job, true); close() } },
  }
}

function look(toast: ToastModel): Look {
  const { job, kind } = toast
  const close = () => dismissToast(toast.id)
  if (isAudioExportJob(job) && (kind === 'err' || kind === 'done' || kind === 'wait')) {
    const ex = exportLook(job)
    const palette = ex.tone === 'err' ? FAIL_COLOR : ex.tone === 'warn' ? WAIT_COLOR : DONE_COLOR
    const tint = ex.tone === 'err' ? 'var(--kit-errf-12)' : ex.tone === 'warn' ? 'var(--kit-warn-12)' : 'var(--kit-ok-12)'
    const ctas = exportCtas(job, close)
    return {
      title: ex.title, sub: ex.sub, note: ex.note, noteColor: ex.tone === 'err' ? 'var(--kit-err-text)' : 'var(--kit-warn-body)', bar: ex.phase === 'dl' ? ex.bar : null,
      tint, iconBg: tint, ring: palette,
      icon: ex.phase === 'dl' ? 'document-download' : ex.phase === 'blocked' ? 'info-circle' : ex.phase === 'fail' ? 'warning-2' : undefined,
      ctas: ex.actions.map((id) => ctas[id]),
    }
  }
  if (kind === 'err') {
    return {
      title: `${job.title} dừng lại`, sub: jobError(job), tint: 'var(--kit-errf-12)', iconBg: 'var(--kit-errf-12)', ring: FAIL_COLOR,
      ctas: [
        { icon: 'refresh-2', label: 'Thử lại', primary: true, run: async () => { await retryJob(job); close() } },
        { icon: 'arrow-right-2', label: 'Chi tiết', run: () => { close(); dispatchDock({ type: 'open' }) } },
      ],
    }
  }
  if (kind === 'wait') {
    return {
      title: job.title, sub: job.step ?? job.result?.sub ?? '', tint: 'rgba(246,203,140,0.12)', iconBg: 'rgba(246,203,140,0.12)', ring: WAIT_COLOR,
      ctas: resultCtas(job, close),
    }
  }
  const agent = job.agent
  return {
    title: job.result?.title ?? job.title, sub: job.result?.sub ?? '',
    tint: agent ? agentAura(agent, 0.16) : 'rgba(166,237,193,0.12)',
    iconBg: agent ? `radial-gradient(circle, ${agentAura(agent, 0.3)}, rgba(20,20,22,0) 70%)` : 'rgba(166,237,193,0.12)',
    ring: agent ? AGENT_PALETTE[agent].c1 : DONE_COLOR,
    ctas: resultCtas(job, close),
  }
}

function ToastIcon({ job, kind, icon, color }: { job: Job; kind: ToastModel['kind']; icon?: string; color: string }) {
  if (icon) return <Icon name={icon} size={18} style={{ color }} />
  if (kind === 'err') return <Icon name="warning-2" size={18} style={{ color: FAIL_COLOR }} />
  if (kind === 'wait') return <Icon name="info-circle" size={18} style={{ color: WAIT_COLOR }} />
  if (!job.agent) return <TickMark size={18} />
  return (
    <>
      <span className="jd-orb" aria-hidden="true"
        style={{ width: 22, height: 22, background: `var(--orb-${job.agent})`, boxShadow: agentOrbGlow(job.agent, 14, 0.6) }} />
      <span style={{
        position: 'absolute', right: -2, bottom: -2, width: 15, height: 15, borderRadius: '50%', background: DONE_COLOR,
        boxShadow: '0 0 0 2px #1C1C1F', display: 'flex', alignItems: 'center', justifyContent: 'center',
      }}>
        <TickMark size={9} color="#0B0B0D" stroke={3.4} />
      </span>
    </>
  )
}

export function JobToast({ toast, front, hover }: { toast: ToastModel; front: boolean; hover: boolean }) {
  const view = look(toast)
  const left = Math.max(0, toast.left)
  return (
    <div className={`jd-tw${toast.leaving ? ' jd-leaving' : ''}`}>
      <div className={toast.leaving ? 'jd-toast-out' : 'jd-toast-in'} style={{ pointerEvents: 'auto' }}>
        <div className={front || hover ? 'jd-tfront' : 'jd-tback'}>
          <div className="jd-glass" style={{ position: 'relative', borderRadius: 18, padding: '12px 12px 13px', display: 'flex', gap: 12, overflow: 'hidden' }}>
            <div aria-hidden="true" style={{ position: 'absolute', inset: 0, background: `radial-gradient(120% 90% at 0% 0%, ${view.tint}, transparent 55%)`, pointerEvents: 'none' }} />
            <span style={{ position: 'relative', width: 34, height: 34, borderRadius: '50%', background: view.iconBg, display: 'flex', alignItems: 'center', justifyContent: 'center', flex: '0 0 auto' }}>
              <ToastIcon job={toast.job} kind={toast.kind} icon={view.icon} color={view.ring} />
            </span>
            <div style={{ position: 'relative', flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 2 }}>
              <span style={{ fontSize: 13.5, fontWeight: 600, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', paddingRight: 28 }}>{view.title}</span>
              <span style={{ fontSize: 12, color: '#A0A0AB' }}>{view.sub}</span>
              {view.note && <span style={{ fontSize: 12, color: view.noteColor ?? '#E9D9BD', lineHeight: 1.45 }}>{view.note}</span>}
              {view.bar !== null && view.bar !== undefined && (
                <span aria-hidden="true" style={{ marginTop: 6, height: 4, borderRadius: 2, background: 'rgba(255,255,255,0.07)', overflow: 'hidden', display: 'block' }}>
                  <span style={{ display: 'block', height: '100%', width: `${view.bar}%`, borderRadius: 2, background: DONE_COLOR, transition: 'width .3s linear' }} />
                </span>
              )}
              {view.ctas.length > 0 && (
                <div style={{ display: 'flex', gap: 6, marginTop: 8 }}>
                  {view.ctas.map((cta) => <CtaButton key={cta.label} cta={cta} />)}
                </div>
              )}
            </div>
            <button type="button" className="jd-ibtn jd-cdbtn" onClick={() => dismissToast(toast.id)} aria-label="Đóng thông báo"
              title={hover ? 'Đã dừng tự đóng' : `Tự đóng sau ${Math.ceil(left / 1000)} giây`}
              style={{
                position: 'absolute', top: 9, right: 9, width: 28, height: 28, borderRadius: '50%', border: 'none',
                background: 'transparent', color: '#8E8E99', display: 'flex', alignItems: 'center', justifyContent: 'center',
              }}>
              <svg width="28" height="28" viewBox="0 0 28 28" aria-hidden="true" style={{ position: 'absolute', inset: 0, transform: 'rotate(-90deg)' }}>
                <circle cx="14" cy="14" r="12" fill="none" stroke="rgba(255,255,255,0.07)" strokeWidth="1.6" />
                <circle className="jd-cdr" cx="14" cy="14" r="12" fill="none" stroke={view.ring} strokeWidth="1.6" strokeLinecap="round"
                  strokeDasharray={CIRCUMFERENCE} strokeDashoffset={(CIRCUMFERENCE * (1 - left / TOAST_MS)).toFixed(2)}
                  style={{ opacity: hover ? 0.3 : 1 }} />
              </svg>
              <Icon name="add" size={14} style={{ position: 'relative', transform: 'rotate(45deg)' }} />
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
