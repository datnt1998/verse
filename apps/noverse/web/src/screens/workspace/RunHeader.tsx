// RunHeader — tiêu đề truyện + hành động (Tạm dừng/Tiếp tục · /export · /reopen)
// và hàng chip trạng thái (Main.dc.html). Nhãn lấy từ 06-data-contract.md › Snapshot → UI:
// mã enum (`COMPLETE`, `complete`, `auto`…) dịch qua enumLabel/translateHost, không in mã thô.
import type { ReactNode } from 'react'
import { storyLabel } from '../../shell/labels'
import type { Snapshot } from '../../api/types'
import { t } from '../../i18n/vi'
import { translateHost } from '../../i18n/hostStrings'
import { enumLabel } from '../../i18n/enums'
import { Button } from '@noverse/ui'
import { Chip, type Tone } from '@noverse/ui'
import { Icon } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import { isCompleted, isGate } from './GateCard'

const FLOW_TONE: Record<string, Tone> = { rewriting: 'accent', polishing: 'accent', steering: 'accent', reviewing: 'info' }
const IN_PROGRESS_LABEL: Record<string, string> = {
  writing: t.workspace.writing,
  rewriting: t.workspace.rewriting,
  polishing: t.workspace.polishing,
}

/**
 * Nhãn StatusChip: host phát mã enum một từ (`COMPLETE`, `READY`…) hoặc câu tiếng Trung
 * (`写第 6 章`). Mã enum đi qua enumLabel; câu giữ nguyên đường translateHost.
 */
export function statusText(statusLabel: string, runtimeState: string): string {
  const raw = (statusLabel || runtimeState).trim()
  if (raw === '') return ''
  return /^[A-Za-z_]+$/.test(raw) ? enumLabel('status', raw) : translateHost(raw)
}

/** Màu StatusChip theo nhãn (Main.dc.html + 22-brief 27/10): nền --tint-* 10%, viền cùng màu 30%. */
export function statusTone(snapshot: Snapshot | null, running: boolean, paused: boolean): Tone {
  // Cổng duyệt (“Thẩm duyệt”) và nhịp chờ phiên (“Đang chờ bạn”) đều cam.
  const wait = snapshot?.waitingFor
  if (wait === 'cocreate' || wait === 'confirm' || wait === 'gate') return 'warn'
  if (running) return 'accent'
  if (isCompleted(snapshot)) return 'ok'
  if (paused || snapshot?.runtimeState === 'pausing' || snapshot?.runtimeState === 'paused') return 'warn'
  // Nhãn đã dịch (qua statusText → enumLabel/translateHost) quyết màu:
  // chờ bạn & thẩm duyệt → cam; lỗi → đỏ; sẵn sàng/rảnh → tím như artboard “Sẵn sàng”.
  const label = snapshot ? statusText(snapshot.statusLabel, snapshot.runtimeState) : ''
  if (label === t.workspace.waiting || label === t.status.review || label === t.status.paused || label === t.status.pausing) return 'warn'
  if (label === t.status.failed) return 'err'
  if (label === '') return wait ? 'warn' : 'accent'
  if (label === t.status.ready || label === t.status.idle) return 'accent'
  return wait ? 'warn' : 'neutral'
}

/**
 * G5 (19-gap-closure.md): chip trạng thái đọc `runtimeState` + `waitingFor`.
 * Đang chờ người dùng (cổng duyệt · đồng sáng tác · xác nhận nhập · chờ lệnh) → “Đang chờ bạn”;
 * đang chạy → “Đang chạy”; còn lại dịch nhãn host như trước.
 */
export function statusChip(snapshot: Snapshot | null): string {
  if (!snapshot) return ''
  // Main.dc.html: cổng duyệt và trạng thái rảnh giữ nhãn host (“Thẩm duyệt”, “Sẵn sàng”);
  // “Đang chờ bạn” chỉ cho các nhịp chờ phiên (đồng sáng tác · xác nhận nhập).
  if (snapshot.waitingFor === 'cocreate' || snapshot.waitingFor === 'confirm') return t.workspace.waiting
  if (snapshot.waitingFor === 'gate') return statusText(snapshot.statusLabel, snapshot.runtimeState) || t.workspace.waiting
  if (snapshot.isRunning) return t.status.running
  const label = statusText(snapshot.statusLabel, snapshot.runtimeState)
  if (label) return label
  if (snapshot.waitingFor) return t.workspace.waiting
  return ''
}

/** Nhãn 3 chip enum còn lại của hàng chip — luôn là chữ Việt, không in mã thô. */
export function runChips(snapshot: Snapshot | null): { phase: string; phaseColor: string; flow: string; flowColor: string; mode: string; modeColor: string } {
  // Gói P · P1: 3 giai đoạn lập kế hoạch (init/premise/outline) hiện một nhãn “Lập kế hoạch”.
  const phase = snapshot?.plan ? t.plan.phase : snapshot ? enumLabel('phase', snapshot.phase) : ''
  // Main.dc.html: “Lập kế hoạch”/“Kế hoạch xong” xanh Kiến trúc sư; “Quy trình” tím;
  // “Nghiệm thu từng chương” cam. Còn lại trắng.
  return {
    phase,
    phaseColor: snapshot?.plan || phase === t.phase?.ready ? 'var(--color-info)' : '',
    flow: snapshot ? enumLabel('flow', snapshot.flow) : '',
    flowColor: 'var(--color-acc-soft)',
    mode: snapshot ? enumLabel('advanceMode', snapshot.advanceMode) : '',
    modeColor: snapshot?.advanceMode === 'review' ? 'var(--color-warn)' : '',
  }
}

/**
 * Sáng tác đang dừng nhưng còn việc để chạy tiếp → header phải có nút “Tiếp tục”.
 * Người dùng báo lỗi 27/10: dừng vì lỗi (`runtimeState=error`, hoặc host đã hạ về
 * `idle` sau lỗi) không có nút nào, chỉ còn lời nhắc gõ lệnh. `recoveryLabel` là
 * dấu hiệu host tự phát khi có điểm khôi phục (`Resume()` thật sự chạy lại); cổng
 * duyệt và truyện đã xong có hành động riêng nên không mượn nút này.
 */
export function canResume(snapshot: Snapshot | null, running: boolean, paused: boolean): boolean {
  if (!snapshot || running) return false
  if (paused) return true
  if (isCompleted(snapshot) || isGate(snapshot)) return false
  return snapshot.recoveryLabel.trim() !== ''
}

export function RunHeader({ snapshot, story, running, paused, writable, onPause, onResume, onExport, onReopen, onToggleReview, busy = false, pending = false, success = false }: {
  snapshot: Snapshot | null
  /** slug truyện — dùng khi chưa có snapshot */
  story: string
  running: boolean
  paused: boolean
  writable: boolean
  onPause: () => void
  onResume: () => void
  onExport: () => void
  onReopen: () => void
  onToggleReview: () => void
  busy?: boolean
  pending?: boolean
  success?: boolean
}) {
  const title = snapshot?.bookTitle || storyLabel({ story })
  const completed = isCompleted(snapshot)
  const status = snapshot ? statusChip(snapshot) : '…'
  const resume = canResume(snapshot, running, paused)
  const chipTone = statusTone(snapshot, running, paused)
  const { phase, phaseColor, flow, flowColor, mode, modeColor } = runChips(snapshot)
  const inProgress = snapshot?.inProgressChapter ?? 0
  const inProgressLabel = snapshot ? IN_PROGRESS_LABEL[snapshot.flow] : undefined
  const permit = snapshot?.advancePermitChapter ?? 0
  const recovery = snapshot?.recoveryLabel ? translateHost(snapshot.recoveryLabel) : ''

  const chipButton = (label: string, onPress: () => void, children: ReactNode) => (
    <button type="button" onClick={onPress} aria-label={label} title={label}
      style={{ border: 'none', background: 'none', padding: 0, display: 'inline-flex', borderRadius: 7 }}>
      {children}
    </button>
  )

  return (
    <div style={{ flex: '0 0 auto', display: 'flex', flexDirection: 'column', gap: 8 }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
        <h1 title={title} style={{
          flex: 1, minWidth: 0, margin: 0, fontSize: 17, fontWeight: 600, lineHeight: 1.35,
          letterSpacing: '-0.015em', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
        }}>{title}</h1>
        {running && writable && (
          <Button icon={<Icon name="pause" size={14} />} kbd="Esc" onClick={onPause} disabled={busy} pending={pending} success={success} style={{ height: 32, paddingInline: 11, borderRadius: 8, gap: 6, fontWeight: 400 }}>{t.workspace.pause}</Button>
        )}
        {resume && writable && (
          <Button icon={<Icon name="play" size={14} />} kbd="Esc" onClick={onResume} disabled={busy} pending={pending} success={success} style={{ height: 32, paddingInline: 11, borderRadius: 8, gap: 6, fontWeight: 400 }}>{t.workspace.resume}</Button>
        )}
        {completed && (
          <div style={{ flex: '0 0 auto', display: 'flex', gap: 6 }}>
            <Button icon={<Icon name="export-curve" size={14} />} onClick={onExport} style={{ height: 32, paddingInline: 11, borderRadius: 8, gap: 6, fontWeight: 400 }}>{t.workspace.exportBook}</Button>
            {writable && (
              <Button variant="primary" icon={<Icon name="edit-2" size={14} />} onClick={onReopen} style={{ height: 32, paddingInline: 12, borderRadius: 8, gap: 6 }}>{t.workspace.reopen}</Button>
            )}
          </div>
        )}
      </div>
      <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 6 }}>
        {/* Main.dc.html › phase=plan: chip của giai đoạn lập kế hoạch chỉ có Giai đoạn + Chế độ. */}
        <Chip tone={chipTone} icon={running ? <Spinner size={11} color="currentColor" />
          : paused ? <Icon name="pause" size={14} /> : undefined}
          label={<b style={{ fontWeight: 500, color: 'inherit' }}>{status}</b>} />
        {phase !== '' && (
          <Chip icon={<Icon name="flag" size={14} />} label={t.workspace.phase} value={phase}
            valueColor={phaseColor || undefined} />
        )}
        {flow !== '' && !snapshot?.plan && !snapshot?.pendingAsk &&
          !(snapshot?.runtimeState === 'idle' && !isCompleted(snapshot)) && (
          <Chip icon={<Icon name="routing-2" size={14} />} label={t.workspace.flow} value={flow}
            valueColor={flowColor || undefined} />
        )}
        {mode !== '' && (writable && !running && snapshot
          ? chipButton(`${t.workspace.mode}: ${mode}`, onToggleReview,
            <Chip icon={<Icon name="magicpen" size={14} />} label={t.workspace.mode} value={mode}
              valueColor={modeColor || undefined} />)
          : <Chip icon={<Icon name="magicpen" size={14} />} label={t.workspace.mode} value={mode}
            valueColor={modeColor || undefined} />)}
        {running && inProgress > 0 && inProgressLabel && !completed && (
          <Chip icon={<Icon name="edit-2" size={14} />} label={inProgressLabel} value={t.workspace.chapter(inProgress)} />
        )}
        {permit > 0 && (
          <Chip icon={<Icon name="tick-circle-bulk" size={14} />} label={t.workspace.approved(permit)} tone="ok" />
        )}
        {recovery !== '' && (
          <Chip tone="accent" icon={<Icon name="refresh-2" size={14} />} label={recovery} />
        )}
      </div>
    </div>
  )
}
