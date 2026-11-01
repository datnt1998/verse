// Thẻ Máy đọc dùng chung — thay InstallCard ở Bản đọc (Transcript), Phòng thu (Studio), Nghe (Listen)
// và là thẻ đầu mục Tuỳ chỉnh › Giọng đọc (10-audio.md §8.1, §8.3).
// Mọi câu chữ đi qua `machineView`; nơi đây chỉ dựng khung, màu và nút.
import { useCallback, type CSSProperties, type ReactNode } from 'react'
import { getTtsStatus, installTts } from '../../api/client'
import { invalidate, keys, useResource } from '../../data'
import { dispatchDock } from '../../shell/jobs/store'
import { navigate, paths } from '../../router'
import { Button, Icon, Spinner, toast } from '@noverse/ui'
import {
  machineView,
  type MachineAction, type MachineActionKind, type MachineStatus, type MachineTone, type MachineView,
} from './machine'

/** Trạng thái máy đọc dùng chung một khoá cache: mọi màn thấy cùng một con số. */
export function useMachineStatus(waitN?: number) {
  const resource = useResource([...keys.tts(), 'status'], ({ signal }) => getTtsStatus(signal))
  const status = resource.data as MachineStatus | undefined
  return { status, view: machineView(status, { waitN }), refresh: () => void resource.refresh() }
}

/** Nút trên thẻ: Thử lại/Chuẩn bị gửi lệnh đến máy đọc, “Mở Nâng cao”/“Xem Giọng đọc” sang Tuỳ chỉnh. */
export function useMachineCardActions() {
  const onAction = useCallback(async (kind: MachineActionKind) => {
    if (kind === 'advanced') { navigate(paths.settings('adv')); return }
    if (kind === 'settings') { navigate(paths.settings('voice')); return }
    try {
      await installTts()
      dispatchDock({ type: 'open' })
      toast('Đã gửi lệnh chuẩn bị bộ đọc cho máy đọc.', 'ok', { label: 'Mở việc chạy nền', onClick: () => dispatchDock({ type: 'open' }) })
      void invalidate(keys.tts()); void invalidate(keys.jobs())
    } catch {
      toast('Chưa gửi được lệnh cho máy đọc. Thử lại sau ít phút.', 'err')
    }
  }, [])
  return { onAction }
}

const TONE: Record<MachineTone, { bg: string; bd: string; iconBg: string; iconFg: string; chipBg: string; chipFg: string }> = {
  neutral: { bg: 'var(--color-card)', bd: 'var(--line-2)', iconBg: 'var(--line-1)', iconFg: 'var(--color-fg-3)', chipBg: 'var(--line-1)', chipFg: 'var(--color-fg-3)' },
  info: { bg: 'var(--color-card)', bd: 'var(--kit-info-25)', iconBg: 'var(--kit-info-12)', iconFg: 'var(--color-info)', chipBg: 'var(--tint-info)', chipFg: 'var(--color-info)' },
  danger: { bg: 'var(--kit-errf-5)', bd: 'var(--kit-errf-30)', iconBg: 'var(--kit-errb-16)', iconFg: 'var(--color-err)', chipBg: 'var(--tint-err)', chipFg: 'var(--kit-err-soft)' },
  warn: { bg: 'var(--color-card)', bd: 'var(--kit-warn-25)', iconBg: 'var(--kit-warn-12)', iconFg: 'var(--color-warn)', chipBg: 'var(--tint-warn)', chipFg: 'var(--kit-warn-text)' },
  ok: { bg: 'var(--color-card)', bd: 'var(--line-2)', iconBg: 'var(--kit-ok-12)', iconFg: 'var(--color-ok)', chipBg: 'var(--tint-ok)', chipFg: 'var(--color-ok)' },
}

const PRIMARY_ACTIONS: MachineActionKind[] = ['retry', 'prepare']

/** Màu chip nhỏ (Cảnh báo / Bắt sửa) trong tiêu đề thẻ. */
function chipStyle(tone: 'warn' | 'error'): CSSProperties {
  return tone === 'error'
    ? { background: 'var(--tint-err)', color: 'var(--kit-err-soft)' }
    : { background: 'var(--tint-warn)', color: 'var(--kit-warn-text)' }
}

function CardChip({ text, tone }: { text: string; tone: 'warn' | 'error' }) {
  return (
    <span style={{
      height: 20, padding: '0 7px', borderRadius: 6, fontSize: 11.5, fontWeight: 500,
      display: 'inline-flex', alignItems: 'center', whiteSpace: 'nowrap', ...chipStyle(tone),
    }}>{text}</span>
  )
}

function StatusPill({ tone, children }: { tone: MachineTone; children: ReactNode }) {
  const colors = TONE[tone]
  return (
    <span style={{
      display: 'inline-flex', alignItems: 'center', gap: 6, height: 22, padding: '0 8px',
      borderRadius: 999, fontSize: 12, fontWeight: 500, whiteSpace: 'nowrap',
      background: colors.chipBg, color: colors.chipFg,
    }}>
      <i aria-hidden="true" style={{ width: 6, height: 6, borderRadius: '50%', background: 'currentColor' }} />
      {children}
    </span>
  )
}

/** Ô biểu tượng 36 px (thẻ đầu màn) / 42 px (Tuỳ chỉnh). */
function Tile({ view, size }: { view: MachineView; size: number }) {
  const colors = TONE[view.tone]
  const inner = size >= 40 ? 20 : 18
  if (view.kind === 'offline') {
    return (
      <span aria-hidden="true" style={{
        flex: '0 0 auto', width: size, height: size, borderRadius: size >= 40 ? 12 : 10,
        background: 'var(--line-1)', display: 'flex', alignItems: 'center', justifyContent: 'center',
      }}>
        <span style={{ width: 9, height: 9, borderRadius: '50%', background: 'var(--color-fg-4)', boxShadow: '0 0 0 3px var(--line-1)' }} />
      </span>
    )
  }
  return (
    <span aria-hidden="true" style={{
      flex: '0 0 auto', width: size, height: size, borderRadius: size >= 40 ? 12 : 10,
      background: colors.iconBg, color: colors.iconFg, display: 'flex', alignItems: 'center', justifyContent: 'center',
    }}>
      {view.spinning ? <Spinner size={size >= 40 ? 18 : 16} color={colors.iconFg} /> : <Icon name={view.icon} size={inner} />}
    </span>
  )
}

function ProgressBar({ progress }: { progress: { percent: number | null; label: string } }) {
  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginTop: 4 }}>
      {progress.percent !== null && (
        <span role="progressbar" aria-label="Chuẩn bị bộ đọc" aria-valuemin={0} aria-valuemax={100} aria-valuenow={progress.percent}
          style={{ flex: '1 1 auto', maxWidth: 420, height: 6, borderRadius: 3, background: 'var(--line-3)', overflow: 'hidden' }}>
          <span style={{ display: 'block', width: `${progress.percent}%`, height: '100%', borderRadius: 3, background: 'linear-gradient(90deg, var(--kit-info-dot), var(--kit-info-fg))' }} />
        </span>
      )}
      <span className="num" style={{ fontSize: 12, color: 'var(--color-fg-4)', whiteSpace: 'nowrap' }}>{progress.label}</span>
    </div>
  )
}

function Actions({ actions, onAction }: { actions: MachineAction[]; onAction?: (kind: MachineActionKind) => void }) {
  if (actions.length === 0) return null
  return (
    <div style={{ flex: '0 0 auto', display: 'flex', gap: 8, alignSelf: 'center' }}>
      {actions.map((action) => (
        <Button key={action.label} size="sm" variant={PRIMARY_ACTIONS.includes(action.kind) ? 'primary' : 'ghost'}
          onClick={() => onAction?.(action.kind)}>{action.label}</Button>
      ))}
    </div>
  )
}

export type MachineCardProps = {
  view: MachineView
  /** `inline` = thẻ đầu màn Bản đọc/Phòng thu/Nghe · `settings` = thẻ trong Tuỳ chỉnh › Giọng đọc */
  variant?: 'inline' | 'settings'
  /** Tuỳ chỉnh: dòng “Bước i/6 · …” của lúc đang chuẩn bị (`machineStepLine(status)`) */
  stepLine?: string
  /** Tuỳ chỉnh: 4 ô số liệu khi máy đọc sẵn sàng */
  tiles?: { label: string; value: string }[]
  onAction?: (kind: MachineActionKind) => void
  style?: CSSProperties
}

export function MachineCard({ view, variant = 'inline', stepLine, tiles, onAction, style }: MachineCardProps) {
  const colors = TONE[view.tone]
  const settings = variant === 'settings'
  const actions = settings ? view.actions : view.cardActions
  const sentence = settings && view.kind === 'provisioning' ? stepLine ?? view.sentence : settings ? view.sentence : view.cardText
  if (!settings && view.ready) return null
  return (
    <section role="status" aria-label="Máy đọc" style={{
      display: 'flex', flexDirection: 'column', gap: settings ? 14 : 0,
      padding: settings ? '18px 20px' : '14px 16px', borderRadius: settings ? 16 : 14,
      background: colors.bg, border: `1px solid ${colors.bd}`, ...style,
    }}>
      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 14 }}>
        <Tile view={view} size={settings ? 42 : 36} />
        <div style={{ flex: '1 1 auto', minWidth: 0, display: 'flex', flexDirection: 'column', gap: 4 }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap' }}>
            <b style={{ fontSize: settings ? 15.5 : 14, fontWeight: 600, color: 'var(--color-fg)' }}>
              {settings ? view.settingsTitle ?? view.machineName : view.title}
            </b>
            {settings && <StatusPill tone={view.tone}>{view.statusLabel}</StatusPill>}
            {view.chip && <CardChip text={view.chip.text} tone={view.chip.tone} />}
          </div>
          <span style={{ fontSize: 13, color: 'var(--color-fg-3)', lineHeight: 1.55 }}>{sentence}</span>
          {view.progress && !settings && <ProgressBar progress={view.progress} />}
        </div>
        <Actions actions={actions} onAction={onAction} />
      </div>

      {settings && view.steps && <StepList steps={view.steps} />}
      {settings && view.ready && tiles && tiles.length > 0 && (
        <div style={{ marginLeft: 56, display: 'grid', gridTemplateColumns: 'repeat(4, minmax(0, 1fr))', gap: 8 }}>
          {tiles.map((tile) => (
            <div key={tile.label} style={{
              display: 'flex', flexDirection: 'column', gap: 3, padding: '10px 12px', borderRadius: 12,
              background: 'var(--color-surface-0)', border: '1px solid var(--line-1)', minWidth: 0,
            }}>
              <span style={{ fontSize: 11.5, color: 'var(--color-fg-4)' }}>{tile.label}</span>
              <b title={tile.value} style={{ fontSize: 13, fontWeight: 600, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>{tile.value}</b>
            </div>
          ))}
        </div>
      )}
      {settings && view.how && view.how.length > 0 && (
        <ol style={{ margin: '0 0 0 56px', paddingLeft: 18, display: 'flex', flexDirection: 'column', gap: 4, fontSize: 13, color: 'var(--color-fg-2)', lineHeight: 1.55 }}>
          {view.how.map((line) => <li key={line}>{line}</li>)}
        </ol>
      )}
      {settings && view.kind === 'provisioning' && (
        <span style={{ marginLeft: 56, fontSize: 12.5, color: 'var(--color-fg-4)', lineHeight: 1.5 }}>
          Lần đầu mất khoảng 10 phút. Trong lúc này vẫn soạn và chốt bản đọc được; chương bấm Tạo giọng sẽ chờ máy đọc.
        </span>
      )}
    </section>
  )
}

function StepList({ steps }: { steps: NonNullable<MachineView['steps']> }) {
  return (
    <ol aria-label="Các bước chuẩn bị bộ đọc" style={{ listStyle: 'none', margin: '0 0 0 56px', padding: 0, display: 'flex', flexDirection: 'column' }}>
      {steps.map((step) => (
        <li key={step.key} style={{ display: 'flex', flexDirection: 'column', gap: 6, padding: '2px 0' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 10, minHeight: 32, fontSize: 13, color: step.state === 'now' ? 'var(--color-fg)' : step.state === 'done' ? 'var(--color-fg-3)' : 'var(--color-fg-4)' }}>
            <span style={{ width: 16, height: 16, display: 'flex', alignItems: 'center', justifyContent: 'center', flex: 'none' }}>
              {step.state === 'done' && <Icon name="tick-circle-bulk" size={16} style={{ color: 'var(--color-ok)' }} />}
              {step.state === 'now' && <Spinner size={13} color="var(--color-info)" />}
              {step.state === 'wait' && <span aria-hidden="true" style={{ width: 9, height: 9, borderRadius: '50%', border: '1.5px solid var(--line-5)' }} />}
            </span>
            <span style={{ flex: '1 1 auto', fontWeight: step.state === 'now' ? 500 : 400 }}>{step.label}</span>
            <span className="num" style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{step.right}</span>
          </div>
          {step.state === 'now' && step.note && (
            <div style={{ marginLeft: 26, display: 'flex', flexDirection: 'column', gap: 5 }}>
              {step.percent !== null && (
                <span role="progressbar" aria-label={step.label} aria-valuemin={0} aria-valuemax={100} aria-valuenow={step.percent}
                  style={{ height: 6, borderRadius: 3, background: 'var(--line-3)', overflow: 'hidden' }}>
                  <span style={{ display: 'block', width: `${step.percent}%`, height: '100%', borderRadius: 3, background: 'linear-gradient(90deg, var(--kit-info-dot), var(--kit-info-fg))' }} />
                </span>
              )}
              <span className="num" style={{ fontSize: 12, color: 'var(--color-fg-4)' }}>{step.note}</span>
            </div>
          )}
        </li>
      ))}
    </ol>
  )
}
