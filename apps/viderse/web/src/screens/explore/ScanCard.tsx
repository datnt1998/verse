// Thẻ "Quét lịch sử" ở cột phải: orb Trinh sát (nhấp khi đang chạy), thanh
// tiến độ n/cap, câu trạng thái (scanView), nút hành động chính (Tạm dừng ·
// Tiếp tục · Quét lại · Quét lịch sử) và listbox Giới hạn (150 · 500 · 1.000 ·
// 2.000 · 5.000 · 10.000 → PATCH scan).
import { useState } from 'react'
import { Icon, toast } from '@noverse/ui'
import { setData } from '@noverse/ui/data'
import { controlRivalScan, setRivalScanCap, type ScanStatus } from '../../api/rivals'
import { vi as t } from '../../i18n/vi'
import { Orb } from '../../components'
import { keys } from '../../data/keys'
import { scanView } from './scanView'

const CAPS = [150, 500, 1000, 2000, 5000, 10000]
const num = new Intl.NumberFormat('vi-VN')

export function ScanCard({ status, rivalId }: { status: ScanStatus; rivalId: string }) {
  const tt = t.screens.explore
  const view = scanView(status)
  const [busy, setBusy] = useState(false)

  const act = (action: 'start' | 'pause' | 'resume' | 'cancel' | 'check') => {
    if (busy) return
    setBusy(true)
    controlRivalScan(rivalId, action)
      .then((next) => setData(keys.rivalScan(rivalId), next))
      .catch((err) => toast.err(err instanceof Error ? err.message : t.common.retry))
      .finally(() => setBusy(false))
  }

  const setCap = (value: string) => {
    const cap = Number(value)
    if (!cap || cap === status.historyCap || busy) return
    setBusy(true)
    setRivalScanCap(rivalId, cap)
      .then((next) => setData(keys.rivalScan(rivalId), next))
      .catch((err) => toast.err(err instanceof Error ? err.message : t.common.retry))
      .finally(() => setBusy(false))
  }

  return (
    <div className="vx-cardbox" style={{ gap: 'var(--vv-s-10)' }}>
      <div className="vx-scanhead">
        <span className="vx-h3"><Orb of="scout" size={14} live={view.live} />{tt.scanTitle}</span>
        <span className="vx-scancount">{view.count}</span>
      </div>
      <div
        role="progressbar"
        aria-label={tt.scanProgress}
        aria-valuemin={0}
        aria-valuemax={view.max}
        aria-valuenow={view.now}
        className="vx-progress"
      >
        <div style={{ width: `${Math.round(view.pct * 100)}%` }} />
      </div>
      <span className="vx-scantext">{view.text}</span>
      <div className="vx-scanrow">
        {view.button && (
          <button
            type="button"
            className="vx-chip"
            style={{ height: 'var(--vv-s-30)' }}
            disabled={busy}
            onClick={() => view.button && act(view.button.action)}
          >
            <Icon name={view.button.icon} size={14} />
            {view.button.label}
          </button>
        )}
        <label className="vx-chip" style={{ border: 'none', position: 'relative', height: 'var(--vv-s-30)' }} aria-label={tt.scanCapLabel}>
          {tt.scanCap} {num.format(status.historyCap)}
          <select
            value={status.historyCap}
            disabled={busy}
            onChange={(e) => setCap(e.target.value)}
            style={{ position: 'absolute', inset: 0, opacity: 0, cursor: 'pointer', font: 'inherit' }}
          >
            {CAPS.map((cap) => <option key={cap} value={cap}>{num.format(cap)}</option>)}
          </select>
          <Icon name="arrow-d" size={12} />
        </label>
      </div>
    </div>
  )
}
