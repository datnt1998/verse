// Tuỳ chỉnh › Ngân sách — Settings.dc.html #budget + 11-api-contract.md §6 (budget.{book_usd,warn_ratio,hard_stop}).
// `budget` là khối ghi nguyên: sửa một trường thì ghi đủ ba trường (server ghép cả khối, không ghép từng trường).
import { useEffect, useState } from 'react'
import type { BudgetConfig, Snapshot } from '../../../api/types'
import { Page, SectionHead, Segc } from '../parts'
import type { SettingsContext } from '../types'

/** $3,20 — dấu phẩy thập phân như artboard. */
export const fmtUsd = (value: number): string => `$${value.toFixed(2).replace('.', ',')}`

/** Chuỗi người dùng gõ → số USD ≥ 0; chấp nhận dấu phẩy; rỗng = 0; không đọc được → null. */
export function parseUsd(text: string): number | null {
  const trimmed = text.trim().replace(',', '.')
  if (trimmed === '') return 0
  if (!/^\d*\.?\d*$/.test(trimmed)) return null
  const value = Number(trimmed)
  return Number.isFinite(value) && value >= 0 ? value : null
}

export type BudgetUsage = { spent: number; limit: number; pct: number; warn: boolean; perChapter: number | null; chaptersLeft: number | null }

/** Thẻ “đã dùng / giới hạn” (artboard): thanh chuyển vàng khi chạm mức báo trước; “ước còn đủ N chương” = (giới hạn − đã dùng) / chi phí trung bình mỗi chương. */
export function budgetUsage(snapshot: Pick<Snapshot, 'totalCostUSD' | 'completedCount'> | null, limit: number, warnRatio: number): BudgetUsage {
  const spent = snapshot?.totalCostUSD ?? 0
  const chapters = snapshot?.completedCount ?? 0
  const perChapter = chapters > 0 && spent > 0 ? spent / chapters : null
  const ratio = limit > 0 ? spent / limit : 0
  return {
    spent, limit, pct: limit > 0 ? Math.min(100, Math.round(ratio * 100)) : 0, warn: limit > 0 && ratio >= warnRatio, perChapter,
    chaptersLeft: limit > 0 && perChapter ? Math.max(0, Math.floor((limit - spent) / perChapter)) : null,
  }
}

const STOPS = [
  { id: false, label: 'Viết xong bước đang làm rồi dừng' },
  { id: true, label: 'Dừng ngay' },
]

function LimitInput({ value, disabled, onCommit }: { value: number; disabled: boolean; onCommit: (usd: number) => void }) {
  const [text, setText] = useState(String(value).replace('.', ','))
  const [last, setLast] = useState(value)
  // Giá trị đổi từ ngoài (Theo dự án / Huỷ) → nạp lại ô.
  useEffect(() => { if (value !== last && parseUsd(text) !== value) setText(String(value).replace('.', ',')); setLast(value) }, [value])
  return (
    <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
      <span style={{ color: '#8E8E99' }}>$</span>
      <input className="st-field num" type="text" inputMode="decimal" value={text} disabled={disabled} aria-label="Giới hạn chi phí (USD)"
        aria-invalid={parseUsd(text) === null || undefined}
        onChange={(event) => {
          setText(event.target.value)
          const parsed = parseUsd(event.target.value)
          if (parsed !== null) onCommit(parsed)
        }}
        style={{ width: 96, textAlign: 'right' }} />
    </span>
  )
}

export function BudgetSection({ ctx }: { ctx: SettingsContext }) {
  const effective: BudgetConfig = { ...ctx.effective.budget, ...ctx.draft.budget }
  const limit = effective.book_usd ?? 0
  const warnRatio = effective.warn_ratio ?? 0.8
  const hardStop = effective.hard_stop ?? false
  const write = (patch: BudgetConfig) => ctx.setField('budget', { book_usd: limit, warn_ratio: warnRatio, hard_stop: hardStop, ...patch })
  const usage = budgetUsage(ctx.snapshot, limit, warnRatio)
  const warnPct = Math.round(warnRatio * 100)

  return (
    <Page>
      <SectionHead title="Ngân sách" />
      {ctx.scope === 'story' && (
        <section className="card st-sec">
          <div style={{ display: 'flex', alignItems: 'baseline', gap: 10 }}>
            <span className="num" style={{ fontSize: 28, fontWeight: 600 }}>{fmtUsd(usage.spent)}</span>
            <span style={{ fontSize: 14, color: '#8E8E99' }}>đã dùng cho truyện này / giới hạn {limit > 0 ? fmtUsd(limit) : 'không giới hạn'}</span>
          </div>
          <div className="st-mbar"><div style={{ height: '100%', width: `${usage.pct}%`, borderRadius: 999, background: usage.warn ? '#F6CB8C' : 'linear-gradient(90deg,#7C93F3,#9A63E3)' }} /></div>
          {usage.perChapter !== null && (
            <span style={{ fontSize: 12.5, color: '#8E8E99' }}>
              Khoảng {fmtUsd(usage.perChapter)} mỗi chương · ước còn đủ cho {usage.chaptersLeft ?? '—'} chương nữa
            </span>
          )}
        </section>
      )}
      <section className="card st-sec">
        <div className="st-srow" style={{ alignItems: 'center' }}>
          <div className="st-stx"><b>Giới hạn chi phí mỗi truyện</b><span>Để 0 nếu không muốn giới hạn.</span></div>
          <LimitInput value={limit} disabled={!ctx.canEdit} onCommit={(usd) => write({ book_usd: usd })} />
        </div>
        <div className="st-srow" style={{ alignItems: 'center' }}>
          <div className="st-stx"><b>Báo trước khi chạm</b><span>Thanh chi phí chuyển vàng ở mức này.</span></div>
          <input type="range" min={50} max={95} step={5} value={warnPct} disabled={!ctx.canEdit} aria-label="Mức báo trước"
            onChange={(event) => write({ warn_ratio: Number(event.target.value) / 100 })}
            style={{ width: 200, margin: 2, accentColor: '#9A63E3' }} />
          <span className="num" style={{ width: 40, textAlign: 'right' }}>{warnPct}%</span>
        </div>
        <div className="st-srow" style={{ alignItems: 'center' }}>
          <div className="st-stx"><b>Khi hết ngân sách</b></div>
          <Segc label="Khi hết ngân sách" value={hardStop} disabled={!ctx.canEdit} options={STOPS} onPick={(next) => write({ hard_stop: next })} />
        </div>
      </section>
    </Page>
  )
}
