import * as stylex from '@stylexjs/stylex'
import { exploreStyles } from './explore.stylex'
// Thẻ "Quét lịch sử" ở cột phải: orb Trinh sát (nhấp khi đang chạy), thanh
// tiến độ n/cap, câu trạng thái (scanView), nút hành động chính (Tạm dừng ·
// Tiếp tục · Quét lại · Quét lịch sử) và listbox Giới hạn (150 · 500 · 1.000 ·
// 2.000 · 5.000 · 10.000 → PATCH scan).
import { useState } from 'react'
import { Button, Icon, toast } from '@noverse/ui'
import { ResponsiveChipSelect } from '../../components/responsive-overlays'
import { setData } from '@noverse/ui/data'
import { controlRivalScan, setRivalScanCap, type ScanStatus } from '../../api/rivals'
import { t } from '../../i18n/explore'
import { Orb } from '../../components'
import { keys } from '../../data/keys'
import { scanView } from './scanView'

const CAPS = [150, 500, 1000, 2000, 5000, 10000]
const num = new Intl.NumberFormat('vi-VN')

export function ScanCard({ status, rivalId, mobile = false }: { status: ScanStatus; rivalId: string; /** tấm thông tin điện thoại: nút chạm 44 px */ mobile?: boolean }) {
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
    <div className={`vx-cardbox ${stylex.props(exploreStyles.cardbox).className}`} style={{ gap: 'var(--vv-s-10)' }}>
      <div className={`vx-scanhead ${stylex.props(exploreStyles.scanhead).className}`}>
        <span className={`vx-h3 ${stylex.props(exploreStyles.h3).className}`}><Orb of="scout" size={14} live={view.live} />{tt.scanTitle}</span>
        <span className={`vx-scancount ${stylex.props(exploreStyles.scancount).className}`}>{view.count}</span>
      </div>
      <div
        role="progressbar"
        aria-label={tt.scanProgress}
        aria-valuemin={0}
        aria-valuemax={view.max}
        aria-valuenow={view.now}
        className={`vx-progress ${stylex.props(exploreStyles.progress).className}`}
      >
        <div {...stylex.props(exploreStyles.progressFill)} style={{ width: `${Math.round(view.pct * 100)}%` }} />
      </div>
      <span className={`vx-scantext ${stylex.props(exploreStyles.scantext).className}`}>{view.text}</span>
      <div className={`vx-scanrow ${stylex.props(exploreStyles.scanrow).className}`}>
        {view.button && (
          <Button size={mobile ? 'lg' : 'sm'} variant="ghost" disabled={busy} onClick={() => view.button && act(view.button.action)} icon={<Icon name={view.button.icon} size={14} />}>
            {view.button.label}
          </Button>
        )}
        <ResponsiveChipSelect label={tt.scanCapLabel} prefix={tt.scanCap} size={mobile ? 'm' : 'sm'} disabled={busy}
          value={String(status.historyCap)} options={CAPS.map((cap) => ({ value: String(cap), label: num.format(cap) }))} onChange={setCap} />
      </div>
    </div>
  )
}
