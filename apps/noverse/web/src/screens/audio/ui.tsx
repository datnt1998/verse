// Mảnh giao diện dùng chung của màn Audio — Eq, switch nhỏ, vòng tiến độ, sóng âm,
// avatar người nói, nút phát (Transcript.dc.html / AudioExport.dc.html inline styles).
import type { CSSProperties, ReactNode } from 'react'
import { Icon } from '@noverse/ui'
import { Spinner } from '@noverse/ui'
import type { AudioCast, CharacterKind } from '../../api/types'
import { KIND_META } from '../../lib/charKind'
import { barHeight, peakBars } from './logic'
import { t } from '../../i18n/vi'
import { Switch } from '@noverse/ui'
import { ProgressRing } from '@noverse/ui'

export function Eq({ color = '#C9A6F5', bars = 4, height = 12, style }: {
  color?: string; bars?: number; height?: number; style?: CSSProperties
}) {
  return (
    <span className="audio-eq" aria-hidden="true"
      style={{ display: 'inline-flex', alignItems: 'center', gap: 2, height, color, ...style }}>
      {Array.from({ length: bars }, (_, index) => <span key={index} />)}
    </span>
  )
}

/** Công tắc mono nhỏ đúng artboard (32×18 / 36×20). */
export function MiniSwitch({ on, onChange, label, width = 32, height = 18, disabled }: {
  on: boolean; onChange: (next: boolean) => void; label: string
  width?: number; height?: number; disabled?: boolean
}) {
  return <Switch checked={on} onChange={onChange} label={label} disabled={disabled}
    size={[width, height]} tone="mono" />
}

/** Thanh chạy gradient (vùng đang hoạt động) — đứng yên khi tạm dừng. */
export function BarRun({ width = '38%', height = 4, running = true, tone = 'run' }: {
  width?: string; height?: number; running?: boolean; tone?: 'run' | 'muted'
}) {
  return (
    <span aria-hidden="true" style={{
      flex: `0 0 ${typeof width === 'number' ? `${width}px` : width}`, height, borderRadius: height,
      background: 'var(--color-surface-5)', overflow: 'hidden', display: 'block',
    }}>
      <span className={running && tone === 'run' ? 'audio-run' : undefined} style={{
        display: 'block', width: '100%', height: '100%',
        background: tone === 'muted' || !running ? 'var(--color-fg-4)' : undefined,
      }} />
    </span>
  )
}

/** Audio progress keeps the artboard's decorative glow behind the shared SVG ring. */
export function Ring({ percent, size = 96, stroke = size === 32 ? 2.8 : 7.5, from = 'var(--ring-voice-start)', to = 'var(--color-accent)', running = true, glow = true, children, label }: {
  percent: number; size?: number; stroke?: number; from?: string; to?: string
  running?: boolean; glow?: boolean; children?: ReactNode; label?: string
}) {
  const value = Math.min(100, Math.max(0, Math.round(percent)))
  return (
    <div role="img" aria-label={label ?? t.audio.ui.daXongPhanTram(value)}
      style={{ position: 'relative', width: size, height: size, flex: '0 0 auto' }}>
      {glow && running && (
        <div aria-hidden="true" className="audio-ringspin" style={{
          position: 'absolute', inset: -6, borderRadius: '50%',
          background: 'conic-gradient(from 0deg, transparent 0deg, rgba(154,99,227,0) 200deg, rgba(201,166,245,.55) 300deg, rgba(93,193,176,.45) 350deg, transparent 360deg)',
          filter: 'blur(8px)',
        }} />
      )}
      <ProgressRing percent={value} size={size} stroke={stroke} track={size === 32 ? 'var(--color-surface-7)' : 'var(--color-surface-5)'}
        gradient={size >= 72 ? [from, to] : undefined} color={to}>{children}</ProgressRing>
    </div>
  )
}

/**
 * Màu viền avatar người nói theo loại vai (Gói S · 04 › *Avatar nhân vật*): bỏ bảng màu
 * riêng từng người của artboard cũ, dùng chung `KIND_META` với nhân vật.
 */
export const kindRing = (kind?: CharacterKind) => KIND_META[kind ?? 'support'].border

const ROOT_TINTS = ['185,208,247', '246,203,140', '166,237,193']

/** Màu theo nhóm giọng (vai dùng chung giọng — sameAs — cùng màu); null = nhân vật phụ / chưa rõ. Giọng chung: tím. */
export function speakerTint(cast: AudioCast | null, key: string, alpha = 1): string | null {
  if (cast?.mode === 'single') return `rgba(201,166,245,${alpha})`
  const speakers = cast?.speakers ?? []
  const speaker = speakers.find((item) => item.id === key || item.name === key)
  if (!speaker) return null
  const roots: string[] = []
  for (const item of speakers) { const root = item.sameAs ?? item.id; if (!roots.includes(root)) roots.push(root) }
  const tint = ROOT_TINTS[roots.indexOf(speaker.sameAs ?? speaker.id)]
  return tint ? `rgba(${tint},${alpha})` : null
}

export type WaveMark = { at: number; label: string; id: string }

/** Sóng âm từ đỉnh server: phần đã phát sáng, cờ cam, kim phát; bấm để tua. Màu theo bộ “nghe” (17-kit.md). */
export function Waveform({ peaks, bars, height, played = 0, color = 'var(--wave-played)', dim = 'var(--wave-unplayed)',
  marks = [], playhead = null, onSeek, label, barGap = 2 }: {
  peaks: number[]; bars: number; height: number; played?: number; color?: string; dim?: string
  marks?: WaveMark[]; playhead?: number | null; onSeek?: (fraction: number) => void
  label?: string; barGap?: number
}) {
  const values = peakBars(peaks, bars)
  const click = (event: React.MouseEvent<HTMLDivElement>) => {
    if (!onSeek) return
    const rect = event.currentTarget.getBoundingClientRect()
    onSeek(Math.min(1, Math.max(0, (event.clientX - rect.left) / rect.width)))
  }
  return (
    <div style={{ position: 'relative' }}>
      <div role={onSeek ? 'slider' : 'img'} aria-label={label} aria-valuenow={onSeek ? Math.round(played * 100) : undefined}
        tabIndex={onSeek ? 0 : undefined} onClick={click}
        style={{ height, display: 'flex', alignItems: 'center', gap: barGap, cursor: onSeek ? 'pointer' : undefined }}>
        {values.map((value, index) => (
          <span key={index} aria-hidden="true" style={{
            flex: 1, minWidth: 1, minHeight: 4, height: barHeight(value), borderRadius: 999,
            background: index / values.length < played ? color : dim,
          }} />
        ))}
      </div>
      {marks.map((mark) => (
        <span key={mark.id} title={mark.label} aria-hidden="true" style={{
          position: 'absolute', top: -6, left: `${Math.min(100, Math.max(0, mark.at * 100))}%`,
          width: 2, height: height + 12, background: 'var(--color-warn)', borderRadius: 1,
        }}>
          <span style={{
            position: 'absolute', top: -4, left: -4, width: 10, height: 10,
            borderRadius: '50%', background: 'var(--color-warn)',
          }} />
        </span>
      ))}
      {playhead !== null && (
        <span aria-hidden="true" style={{
          position: 'absolute', top: -4, left: `${Math.min(100, Math.max(0, playhead * 100))}%`,
          width: 2, height: height + 8, background: 'var(--color-fg)', borderRadius: 1,
        }} />
      )}
    </div>
  )
}

/** Nút nghe tròn: idle → spinner → Eq khi đang phát; chấm góc = đã có giọng. */
export function PlayCircle({ playing, loading, onClick, label, size = 28, tone = 'default', dot }: {
  playing: boolean; loading: boolean; onClick: () => void; label: string
  size?: number; tone?: 'default' | 'primary'; dot?: string
}) {
  return (
    <span style={{ position: 'relative', display: 'inline-flex' }}>
      {dot && (
        <span title={label} style={{
          position: 'absolute', right: -2, top: -2, zIndex: 1, width: 9, height: 9, borderRadius: '50%',
          background: dot, boxShadow: '0 0 0 2px var(--color-bg)',
        }} />
      )}
      <button type="button" className="hov-ghost" onClick={onClick} aria-label={label} title={label}
        style={{
          width: size, height: size, borderRadius: '50%',
          border: tone === 'primary' ? 'none' : '1px solid var(--line-4)',
          background: playing || tone === 'primary' ? 'var(--color-fg)' : 'transparent',
          color: playing || tone === 'primary' ? 'var(--color-bg)' : 'var(--color-fg)',
          display: 'flex', alignItems: 'center', justifyContent: 'center',
        }}>
        {loading
          ? <Spinner size={Math.round(size * 0.43)} />
          : playing
            ? <Eq color="var(--color-bg)" height={Math.round(size * 0.43)} />
            : <Icon name="play" size={Math.round(size * 0.43)} />}
      </button>
    </span>
  )
}

/** Khung gợi ý nhỏ (i-info) dùng ở cột trái và panel. */
export function Note({ children, tone = 'neutral', style }: {
  children: ReactNode; tone?: 'neutral' | 'ok' | 'warn'; style?: CSSProperties
}) {
  const color = tone === 'ok' ? 'var(--color-ok)' : tone === 'warn' ? 'var(--color-warn)' : 'var(--color-fg-3)'
  return (
    <div style={{
      display: 'flex', gap: 8, padding: '10px 12px', borderRadius: 12,
      border: '1px solid var(--line-2)', fontSize: 12, color, ...style,
    }}>
      <Icon name={tone === 'ok' ? 'tick-circle-bulk' : tone === 'warn' ? 'warning-2' : 'info-circle'}
        size={14} style={{ marginTop: 2, flex: '0 0 auto' }} />
      <span style={{ minWidth: 0 }}>{children}</span>
    </div>
  )
}

/** Nhãn nhỏ in hoa (artboard .lbl). */
export function Lbl({ children, style }: { children: ReactNode; style?: CSSProperties }) {
  return <span className="lbl" style={style}>{children}</span>
}

/** Thẻ trạng thái chương trong dòng thời gian 8 chương. */
export function ChapterTile({ children, onClick, title, flex, style, className }: {
  children: ReactNode; onClick?: () => void; title?: string; flex: number
  style?: CSSProperties; className?: string
}) {
  return (
    <button type="button" onClick={onClick} title={title} className={className} style={{
      flex, minWidth: 0, position: 'relative', overflow: 'hidden', borderRadius: 9,
      padding: '0 8px', display: 'flex', alignItems: 'center', gap: 6, fontSize: 11.5,
      ...style,
    }}>{children}</button>
  )
}
