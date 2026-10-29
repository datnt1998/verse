// Panel phải 380 px (Quality.dc.html › aside “Chi tiết”): Tóm tắt (Việc cần làm) hoặc vùng đang chọn
// của ma trận (ô · chương · mục luật) với thẻ ReviewIssue, rồi “Mục hay vi phạm nhất”.
import type { Quality, QualityIssue } from '../../api/types'
import { t } from '../../i18n/vi'
import { paths } from '../../router'
import { Icon } from '@noverse/ui'
import { pickedIssues, topViolated, type QualityPick, type QualityView, type SummaryParts } from './model'
import { ReviewIssue, reviewLevel } from '../../ui/ReviewIssue'

const STATE_COLOR: Record<QualityIssue['state'], string> = {
  open: 'var(--color-fg)', fixed: 'var(--color-ok)', kept: 'var(--color-fg-3)',
}

function summaryText(parts: SummaryParts): string {
  const s = t.quality.summary
  return (parts.block ? s.blocking(parts.block, parts.blockChapters) : s.clear)
    + (parts.look ? s.look(parts.look) : '')
    + (parts.auto && parts.span ? s.auto(parts.auto, parts.span.from, parts.span.to) : '')
}

function PickDetail({ data, issues, pick, project, story }: {
  data: Quality; issues: QualityIssue[]; pick: QualityPick; project: string; story: string
}) {
  const items = pickedIssues(issues, pick)
  const section = data.sections.find((row) => row.ref === pick.ref)
  const chapter = data.chapters.find((row) => row.n === pick.ch)
  const d = t.quality.detail
  let kind: string, title: string, sub: string, open: string, target: number | undefined
  if (pick.ref !== undefined && pick.ch !== undefined) {
    kind = d.cell; title = d.cellTitle(pick.ref, pick.ch); sub = section?.refName ?? ''
    open = d.openChapter(pick.ch); target = pick.ch
  } else if (pick.ch !== undefined) {
    kind = d.chapter; title = t.quality.chapterN(pick.ch); sub = d.chapterSub(t.quality.status[chapter?.status ?? ''] ?? '', items.length)
    open = d.openChapter(pick.ch); target = pick.ch
  } else {
    const ref = pick.ref ?? ''
    kind = d.row; title = ref; sub = d.rowSub(section?.refName ?? '', items.length, new Set(items.map((issue) => issue.ch)).size)
    open = d.openFirst; target = items.length ? Math.min(...items.map((issue) => issue.ch)) : undefined
  }
  return (
    <>
      <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
        <span className="lbl">{kind}</span>
        <span style={{ fontSize: 16, fontWeight: 600 }}>{title}</span>
        <span style={{ fontSize: 12.5, color: 'var(--kit-fg-icon)' }}>{sub}</span>
      </div>
      {items.length === 0 && (
        <div style={{
          padding: '24px 14px', borderRadius: 14, border: '1px dashed var(--kit-line-12)', textAlign: 'center', fontSize: 13, color: 'var(--kit-fg-icon)',
        }}>{d.empty}</div>
      )}
      {items.map((issue) => (
        <div key={issue.id} style={{
          padding: 12, borderRadius: 14, background: 'var(--quality-card)',
          border: `1px solid ${reviewLevel(issue) === 'error' && issue.state === 'open' ? 'var(--kit-errf-25)' : 'var(--line-2)'}`,
          opacity: issue.state === 'open' ? 1 : 0.72,
        }}>
          <ReviewIssue item={issue} variant="quality" frame={false} context={t.quality.chapterN(issue.ch)}
            aside={{ text: t.quality.state[issue.state] ?? '', color: STATE_COLOR[issue.state] }} />
        </div>
      ))}
      <a className="ghost" href={target !== undefined ? `#${paths.reader(project, story, target)}` : undefined} aria-disabled={target === undefined}
        style={{
          height: 34, borderRadius: 9, border: '1px solid var(--line-4)', display: 'flex', alignItems: 'center', justifyContent: 'center',
          gap: 8, fontSize: 13, color: 'var(--color-fg)',
        }}>
        <Icon name="book-1" size={15} />{open}
      </a>
    </>
  )
}

export function Detail({ data, view, issues, pick, summary, project, story, onMatrix }: {
  data: Quality
  view: QualityView
  /** mục đã qua bộ lọc mức + phạm vi */
  issues: QualityIssue[]
  pick: QualityPick
  summary: SummaryParts
  project: string
  story: string
  onMatrix: () => void
}) {
  const top = topViolated(data)
  return (
    <aside aria-label={t.quality.detailLabel} className="scroll" style={{
      borderLeft: '1px solid var(--line-1)', padding: '18px 16px', display: 'flex', flexDirection: 'column', gap: 12,
    }}>
      {view === 'todo' ? (
        <div style={{
          display: 'flex', flexDirection: 'column', gap: 10, padding: 14, borderRadius: 14,
          background: 'var(--quality-card)', border: '1px solid var(--line-2)',
        }}>
          <span className="lbl">{t.quality.summaryLabel}</span>
          <span style={{ fontSize: 14, lineHeight: 1.6, color: 'var(--kit-fg-note)' }}>{summaryText(summary)}</span>
          <button type="button" className="q-tbtn" onClick={onMatrix} style={{ alignSelf: 'flex-start' }}>
            <Icon name="category" size={14} />{t.quality.toMatrix}
          </button>
        </div>
      ) : (
        <PickDetail data={data} issues={issues} pick={pick} project={project} story={story} />
      )}
      {top.length > 0 && (
        <div style={{
          marginTop: 4, padding: 12, borderRadius: 14, background: 'var(--color-surface-0)', border: '1px solid var(--line-1)',
          display: 'flex', flexDirection: 'column', gap: 8,
        }}>
          <span style={{ fontSize: 13, fontWeight: 600 }}>{t.quality.top}</span>
          {top.map((row) => (
            <div key={row.ref} style={{ display: 'flex', alignItems: 'center', gap: 8, fontSize: 12.5 }}>
              <span className="q-chip" style={{ background: 'var(--kit-wash-5)', color: 'var(--color-fg-2)', flex: 'none' }}>{row.ref}</span>
              <span title={row.refName} style={{
                flex: 1, minWidth: 0, color: 'var(--color-fg-3)', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
              }}>{row.refName}</span>
              <span className="num" style={{ color: 'var(--color-fg)' }}>{row.n}</span>
              <span style={{ width: 56, height: 4, borderRadius: 2, background: 'var(--line-1)', flex: 'none' }}>
                <span style={{ display: 'block', height: 4, borderRadius: 2, width: `${row.width}%`, background: 'var(--color-warn)' }} />
              </span>
            </div>
          ))}
          <span style={{ fontSize: 12, color: 'var(--color-fg-4)', lineHeight: 1.5 }}>{t.quality.topNote}</span>
        </div>
      )}
    </aside>
  )
}
