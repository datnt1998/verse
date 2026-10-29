// Ma trận mục luật × chương (Quality.dc.html › view=matrix). Hàng = mục luật của gói, cột = chương
// 1…N + Tổng; ô: viên đỏ Bắt sửa còn mở, cam Cảnh báo còn mở, ✓ đã sửa, “giữ”; ô trống = Đạt.
// Truyện dài: cuộn ngang trong `.q-wrap`, cột đầu `position: sticky`.
import type { CSSProperties } from 'react'
import type { Quality, QualityChapterStatus, QualityIssue } from '../../api/types'
import { t } from '../../i18n/vi'
import { Icon } from '@noverse/ui'
import {
  barHeights, cellIssues, chapterRanges, colTotal, isLong, matrixRows, maxOpenPerChapter, refSource, rowTotal, tally,
  type QualityPick, type QualityScope, type RefSource,
} from './model'

export const SOURCE_CHIP: Record<RefSource, CSSProperties> = {
  master: { background: 'var(--kit-acc-12)', color: 'var(--kit-badge-run)' },
  pack: { background: 'var(--quality-src-pack-bg)', color: 'var(--quality-src-pack-fg)' },
  address: { background: 'var(--frame-info-12)', color: 'var(--frame-info-fg)' },
  user: { background: 'var(--tint-ok)', color: 'var(--quality-src-user-fg)' },
}

export const TONE: Record<'err' | 'warn' | 'ok' | 'muted', string> = {
  err: 'var(--kit-err-soft)', warn: 'var(--kit-warn-soft)', ok: 'var(--color-ok)', muted: 'var(--color-fg-4)',
}

/** Màu chữ trạng thái chương (dòng dưới vạch) và số chương của truyện dài. */
const statusColor = (status: QualityChapterStatus) =>
  status === 'previous' ? 'var(--kit-fg-faint)' : status === 'rewriting' ? 'var(--kit-badge-run)' : 'var(--color-ok)'

export function Matrix({ data, issues, scope, pick, onPick }: {
  data: Quality
  /** mục đã qua bộ lọc mức + phạm vi */
  issues: QualityIssue[]
  scope: QualityScope
  pick: QualityPick
  onPick: (pick: QualityPick) => void
}) {
  const long = isLong(data)
  const rows = matrixRows(data)
  const maxOpen = maxOpenPerChapter(data)
  const n = data.chapters.length
  const grid: CSSProperties = {
    gridTemplateColumns: long ? `220px repeat(${n}, 38px) 56px` : `minmax(0, 1.6fr) repeat(${n}, minmax(0, 1fr)) 64px`,
  }
  const previous = data.chapters.filter((chapter) => chapter.status === 'previous').map((chapter) => chapter.n)
  const grand = issues.filter((issue) => issue.state === 'open').length

  return (
    <div className="scroll" style={{ flex: '1 1 auto', minHeight: 0, padding: '0 24px 20px' }}>
      {long && (
        <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 8, fontSize: 12.5, color: 'var(--kit-fg-icon)' }}>
          <Icon name="info-circle" size={14} />
          <span style={{ flex: 1 }}>{t.quality.longHint(n)}</span>
        </div>
      )}
      <section className={`card${long ? ' q-long' : ''}`} aria-label={t.quality.matrixLabel} style={{ borderRadius: 16, overflow: 'hidden' }}>
        <div className="q-wrap">
          <div className="q-row q-head" style={grid}>
            <span style={{ paddingLeft: 4 }}>{t.quality.refColumn}</span>
            {data.chapters.map((chapter) => {
              const col = issues.filter((issue) => issue.ch === chapter.n)
              const bars = barHeights(tally(col), maxOpen)
              const label = t.quality.status[chapter.status] ?? ''
              const on = pick.ch === chapter.n && pick.ref === undefined
              const fg = long ? statusColor(chapter.status) : chapter.status === 'previous' ? 'var(--kit-fg-icon)' : 'var(--color-fg)'
              return (
                <button key={chapter.n} type="button" className="q-ch" aria-pressed={on}
                  aria-label={t.quality.colAria(chapter.n, label)} onClick={() => onPick({ ch: chapter.n })}>
                  <span className="num" style={{ fontSize: 12.5, color: fg, fontWeight: 600 }}>{chapter.n}</span>
                  <span className="q-bar" aria-hidden="true">
                    <i style={{ height: bars.e, background: 'var(--color-err)' }} />
                    <i style={{ height: bars.w, background: 'var(--color-warn)' }} />
                  </span>
                  <span className="q-st" style={{ fontSize: 10.5, color: statusColor(chapter.status), whiteSpace: 'nowrap' }}>{label}</span>
                </button>
              )
            })}
            <span style={{ textAlign: 'center' }}>{t.quality.total}</span>
          </div>

          {rows.map((row) => {
            const total = rowTotal(tally(issues.filter((issue) => issue.ref === row.ref)), scope)
            const rowOn = pick.ref === row.ref && pick.ch === undefined
            return (
              <div key={row.ref} className="q-row" style={{ ...grid, background: rowOn ? 'var(--kit-acc-5)' : 'transparent' }}>
                <button type="button" className="q-name" aria-pressed={rowOn} onClick={() => onPick({ ref: row.ref })} style={{ paddingLeft: 4 }}>
                  <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                    <span className="q-chip" style={SOURCE_CHIP[refSource(row.ref)]}>{row.ref}</span>
                  </span>
                  <span title={row.refName} style={{
                    fontSize: 12, color: 'var(--kit-fg-icon)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
                  }}>{row.refName}</span>
                </button>
                {data.chapters.map((chapter) => {
                  const here = cellIssues(issues, row.ref, chapter.n)
                  const c = tally(here)
                  const on = pick.ref === row.ref && pick.ch === chapter.n
                  return (
                    <button key={chapter.n} type="button" aria-pressed={on}
                      className={`q-cell${chapter.status === 'previous' ? ' dim' : ''}`}
                      aria-label={here.length ? t.quality.cellAria(row.ref, chapter.n, c.e, c.w, c.f) : t.quality.cellEmpty(row.ref, chapter.n)}
                      onClick={() => onPick({ ref: row.ref, ch: chapter.n })}>
                      {c.e > 0 && <span className="q-pe">{c.e}</span>}
                      {c.w > 0 && <span className="q-pw">{c.w}</span>}
                      {c.f > 0 && scope === 'all' && <span className="q-pf" title={t.quality.fixedTitle}>✓{c.f}</span>}
                      {c.k > 0 && <span className="q-pk" title={t.quality.keptTitle}>{t.quality.keptMark}</span>}
                      {here.length === 0 && <span style={{ color: 'var(--kit-dash)' }}>·</span>}
                    </button>
                  )
                })}
                <span className="num" style={{ textAlign: 'center', fontSize: 12.5, color: TONE[total.tone] }}>{total.text}</span>
              </div>
            )
          })}

          <div className="q-row" style={{ ...grid, borderTop: '1px solid var(--line-3)', background: 'var(--color-scope-bg)' }}>
            <span style={{ paddingLeft: 4, fontSize: 12, color: 'var(--color-fg-3)' }}>{t.quality.perChapter}</span>
            {data.chapters.map((chapter) => {
              const col = issues.filter((issue) => issue.ch === chapter.n)
              const total = colTotal(tally(col), col.length > 0)
              return <span key={chapter.n} className="num" style={{ textAlign: 'center', fontSize: 12.5, color: TONE[total.tone] }}>{total.text}</span>
            })}
            <span className="num" style={{ textAlign: 'center', fontSize: 12.5, color: 'var(--color-fg)', fontWeight: 600 }}>{grand}</span>
          </div>
        </div>
      </section>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px 18px', marginTop: 10, fontSize: 12, color: 'var(--kit-fg-icon)' }}>
        <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><span className="q-pe">1</span>{t.quality.legend.error}</span>
        <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><span className="q-pw">1</span>{t.quality.legend.warning}</span>
        <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><span className="q-pf">✓1</span>{t.quality.legend.fixed}</span>
        <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}><span className="q-pk">{t.quality.keptMark}</span>{t.quality.legend.kept}</span>
        {previous.length > 0 && <span>{t.quality.legendPrevious(chapterRanges(previous))}</span>}
      </div>
    </div>
  )
}
